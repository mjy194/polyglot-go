package handler

import (
	"fmt"
	"net/http"
	"time"

	"polyglot/internal/adapter"
	"polyglot/internal/converter"
	"polyglot/internal/protocol"
	"polyglot/internal/telemetry"
	pb "polyglot/proto/adapter"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StreamProcessorResolver 按 provider 解析出可用的 adapter 流式处理器。
// 动态自注册：adapter 启动时通过 RegisterAccountSource 上报 CallbackAddr，
// 主框架据此建立客户端；若该 provider 尚无 adapter 注册，ok=false（调用方返回 503）。
type StreamProcessorResolver func(c *gin.Context) (adapter.StreamProcessor, bool)

// AnthropicMessages Anthropic Messages API handler。
//
// 链路：校验 → AnthropicToUniversal → adapter.ProcessStream → UniversalToAnthropic。
// 流式请求把 universal 响应实时转成 Anthropic SSE 事件；非流式则收集后一次性返回。
func AnthropicMessages(resolve StreamProcessorResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 解析请求
		var req protocol.AnthropicRequest
		parseSpan := telemetry.Start(c, "protocol.parse", "protocol", "anthropic")
		if err := c.ShouldBindJSON(&req); err != nil {
			parseSpan.EndError(err)
			anthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		parseSpan.End("model", req.Model, "stream", req.Stream)
		telemetry.SetField(c, "model", req.Model)
		telemetry.SetField(c, "stream", req.Stream)

		// 2. 校验
		validateSpan := telemetry.Start(c, "protocol.validate", "protocol", "anthropic")
		if err := req.Validate(); err != nil {
			validateSpan.EndError(err)
			anthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		validateSpan.End()

		// 3. 解析 adapter（按 provider 动态寻址）
		processor, ok := resolve(c)
		if !ok {
			anthropicError(c, http.StatusServiceUnavailable, "api_error",
				"no adapter registered for this backend; service unavailable")
			return
		}

		// 4. Anthropic → Universal
		convertSpan := telemetry.Start(c, "universal.convert_request", "protocol", "anthropic")
		univReq, err := converter.AnthropicToUniversal(&req)
		if err != nil {
			convertSpan.EndError(err)
			anthropicError(c, http.StatusBadRequest, "invalid_request_error",
				fmt.Sprintf("failed to convert request: %v", err))
			return
		}
		convertSpan.End("messages", len(univReq.Messages), "tools", len(univReq.Tools))

		messageID := "msg_" + uuid.New().String()

		// 5. 处理 + 响应
		if req.Stream {
			streamAnthropicResponse(c, processor, univReq, messageID, req.Model)
		} else {
			handleAnthropicNonStream(c, processor, univReq, messageID, req.Model)
		}
	}
}

// handleAnthropicNonStream 非流式：收集全部 universal 响应后一次性转换并返回。
func handleAnthropicNonStream(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest, messageID, model string) {
	ctx := c.Request.Context()

	var responses []*pb.UniversalResponse
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "anthropic", "stream", false)
	err := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		responses = append(responses, resp)
		return nil
	})
	if err != nil {
		span.EndError(err, "responses", len(responses))
		anthropicError(c, http.StatusBadGateway, "api_error",
			fmt.Sprintf("upstream adapter error: %v", err))
		return
	}
	span.End("responses", len(responses))

	// 上游在流中返回了错误响应
	for _, r := range responses {
		if e := r.GetError(); e != nil {
			anthropicError(c, http.StatusBadGateway, "api_error", e.Message)
			return
		}
	}

	convertSpan := telemetry.Start(c, "universal.convert_response", "protocol", "anthropic", "responses", len(responses))
	resp, err := converter.UniversalToAnthropic(messageID, responses)
	if err != nil {
		convertSpan.EndError(err)
		anthropicError(c, http.StatusInternalServerError, "api_error",
			fmt.Sprintf("failed to convert response: %v", err))
		return
	}
	convertSpan.End("content_blocks", len(resp.Content))

	// converter 把 model 写死、且直接透传 finish_reason，这里校正为符合 Anthropic 协议的取值
	resp.Model = model
	finalizeStopReason(resp)

	c.JSON(http.StatusOK, resp)
}

// streamAnthropicResponse 流式：把 universal 响应实时转成 Anthropic SSE 事件序列。
func streamAnthropicResponse(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest, messageID, model string) {
	// SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		anthropicError(c, http.StatusInternalServerError, "api_error", "streaming not supported")
		return
	}

	write := func(s string) {
		_, _ = c.Writer.WriteString(s)
		flusher.Flush()
	}

	// 开场：message_start + ping
	write(protocol.NewMessageStartEvent(messageID, model))
	write(protocol.NewPingEvent())

	// 追踪已开启、尚未关闭的 content block（index -> type），结束时统一补 stop
	openBlocks := make(map[int]string)
	var stopReason string
	var usage protocol.Usage
	var upstreamErr error

	ctx := c.Request.Context()
	adapterStart := time.Now()
	firstEvent := true
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "anthropic", "stream", true)
	streamErr := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		if firstEvent {
			telemetry.Event(c, "adapter.first_event", "protocol", "anthropic", "ttfb_ms", time.Since(adapterStart).Milliseconds())
			firstEvent = false
		}
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			idx := int(r.Chunk.Index)
			if _, started := openBlocks[idx]; !started {
				write(protocol.NewContentBlockStartEvent(idx))
				openBlocks[idx] = "text"
			}
			if r.Chunk.Text != "" {
				write(protocol.NewContentBlockDeltaEvent(idx, r.Chunk.Text))
			}
			if r.Chunk.IsFinal {
				write(protocol.NewContentBlockStopEvent(idx))
				delete(openBlocks, idx)
			}

		case *pb.UniversalResponse_ToolCall:
			idx := int(r.ToolCall.Index)
			write(protocol.NewToolUseBlockStart(idx, r.ToolCall.Id, r.ToolCall.Name))
			openBlocks[idx] = "tool_use"
			if r.ToolCall.Arguments != "" {
				write(protocol.NewInputJsonDelta(idx, r.ToolCall.Arguments))
			}
			write(protocol.NewContentBlockStopEvent(idx))
			delete(openBlocks, idx)
			stopReason = "tool_use"

		case *pb.UniversalResponse_Completion:
			stopReason = mapStopReason(r.Completion.FinishReason)
			usage = protocol.Usage{
				InputTokens:         int(r.Completion.InputTokens),
				OutputTokens:        int(r.Completion.OutputTokens),
				CacheCreationTokens: int(r.Completion.CacheCreationTokens),
				CacheReadTokens:     int(r.Completion.CacheReadTokens),
			}

		case *pb.UniversalResponse_Error:
			write(protocol.NewErrorEvent(errorTypeFor(r.Error.Type), r.Error.Message))
			upstreamErr = fmt.Errorf("upstream error: %s", r.Error.Message)
			return upstreamErr
		}
		return nil
	})
	if streamErr != nil {
		span.EndError(streamErr)
	} else {
		span.End()
	}

	// 收尾：补全所有未关闭的 block
	for idx := range openBlocks {
		write(protocol.NewContentBlockStopEvent(idx))
	}

	if stopReason == "" {
		stopReason = "end_turn"
	}
	write(protocol.NewMessageDeltaEvent(stopReason, usage.InputTokens, usage.OutputTokens))
	write(protocol.NewMessageStopEvent())

	// 流已结束；若中途出错，状态码已无法更改（已写了 200 + 事件），这里仅吞掉，错误已通过 error 事件传达
	_ = streamErr
}

// finalizeStopReason 把 converter 透传的 finish_reason 规整为合法的 Anthropic stop_reason，
// 并在出现 tool_use 内容块时强制为 tool_use。
func finalizeStopReason(resp *protocol.AnthropicResponse) {
	hasToolUse := false
	for _, b := range resp.Content {
		if b.Type == "tool_use" {
			hasToolUse = true
			break
		}
	}
	resp.StopReason = mapStopReason(resp.StopReason)
	if resp.StopReason == "" {
		resp.StopReason = "end_turn"
	}
	if hasToolUse && resp.StopReason != "tool_use" {
		resp.StopReason = "tool_use"
	}
}

// mapStopReason 把 universal/上游 finish_reason 映射为 Anthropic stop_reason。
func mapStopReason(finishReason string) string {
	switch finishReason {
	case "", "stop", "end_turn":
		return "end_turn"
	case "length", "max_tokens":
		return "max_tokens"
	case "tool_calls", "tool_use":
		return "tool_use"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return finishReason
	}
}

// errorTypeFor 把上游错误类型规整为 Anthropic 错误类型（缺省 overloaded_error）。
func errorTypeFor(t string) string {
	if t == "" {
		return "overloaded_error"
	}
	return t
}

// anthropicError 返回 Anthropic 风格的错误 JSON。
func anthropicError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
