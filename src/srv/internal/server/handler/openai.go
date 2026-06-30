package handler

import (
	"encoding/json"
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

// OpenAIChatCompletions OpenAI Chat Completions API handler。
// 链路：校验 → OpenAIToUniversal → adapter.ProcessStream → UniversalToOpenAI。
func OpenAIChatCompletions(resolve StreamProcessorResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 解析请求
		var req protocol.OpenAIRequest
		parseSpan := telemetry.Start(c, "protocol.parse", "protocol", "openai")
		if err := c.ShouldBindJSON(&req); err != nil {
			parseSpan.EndError(err)
			c.JSON(http.StatusBadRequest, protocol.NewOpenAIError(err.Error(), "invalid_request_error"))
			return
		}
		parseSpan.End("model", req.Model, "stream", req.Stream)
		telemetry.SetField(c, "model", req.Model)
		telemetry.SetField(c, "stream", req.Stream)

		// 2. 校验
		validateSpan := telemetry.Start(c, "protocol.validate", "protocol", "openai")
		if err := req.Validate(); err != nil {
			validateSpan.EndError(err)
			c.JSON(http.StatusBadRequest, protocol.NewOpenAIError(err.Error(), "invalid_request_error"))
			return
		}
		validateSpan.End()

		// 3. 解析 adapter（动态寻址）
		processor, ok := resolve(c)
		if !ok {
			c.JSON(http.StatusServiceUnavailable, protocol.NewOpenAIError(
				"no adapter registered for this backend; service unavailable", "api_error"))
			return
		}

		// 4. OpenAI → Universal
		convertSpan := telemetry.Start(c, "universal.convert_request", "protocol", "openai")
		univReq, err := converter.OpenAIToUniversal(&req)
		if err != nil {
			convertSpan.EndError(err)
			c.JSON(http.StatusBadRequest, protocol.NewOpenAIError(
				fmt.Sprintf("failed to convert request: %v", err), "invalid_request_error"))
			return
		}
		convertSpan.End("messages", len(univReq.Messages), "tools", len(univReq.Tools))

		messageID := "chatcmpl-" + uuid.New().String()

		// 5. 处理 + 响应
		if req.Stream {
			streamOpenAIResponse(c, processor, univReq, messageID, req.Model)
		} else {
			handleOpenAINonStream(c, processor, univReq, messageID, req.Model)
		}
	}
}

// handleOpenAINonStream 非流式：收集全部 universal 响应后一次性转换并返回。
func handleOpenAINonStream(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest, messageID, model string) {
	ctx := c.Request.Context()

	var responses []*pb.UniversalResponse
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "openai", "stream", false)
	err := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		responses = append(responses, resp)
		return nil
	})
	if err != nil {
		span.EndError(err, "responses", len(responses))
		c.JSON(http.StatusBadGateway, protocol.NewOpenAIError(
			fmt.Sprintf("upstream adapter error: %v", err), "api_error"))
		return
	}
	span.End("responses", len(responses))

	for _, r := range responses {
		if e := r.GetError(); e != nil {
			c.JSON(http.StatusBadGateway, protocol.NewOpenAIError(e.Message, "api_error"))
			return
		}
	}

	convertSpan := telemetry.Start(c, "universal.convert_response", "protocol", "openai", "responses", len(responses))
	resp, err := converter.UniversalToOpenAI(messageID, model, responses)
	if err != nil {
		convertSpan.EndError(err)
		c.JSON(http.StatusInternalServerError, protocol.NewOpenAIError(
			fmt.Sprintf("failed to convert response: %v", err), "api_error"))
		return
	}
	convertSpan.End("choices", len(resp.Choices))

	// converter 把 Created 留 0、finish_reason 透传；这里校正
	resp.Created = time.Now().Unix()
	finalizeOpenAIFinish(resp)

	c.JSON(http.StatusOK, resp)
}

// streamOpenAIResponse 流式：把 universal 响应实时转成 OpenAI SSE chunk。
func streamOpenAIResponse(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest, messageID, model string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		c.JSON(http.StatusInternalServerError, protocol.NewOpenAIError("streaming not supported", "internal_error"))
		return
	}

	created := time.Now().Unix()
	write := func(s string) {
		_, _ = c.Writer.WriteString(s)
		flusher.Flush()
	}

	// 开场：role chunk
	write(protocol.NewOpenAIStreamStart(messageID, model, created))

	var finishReason string
	toolIdx := -1
	ctx := c.Request.Context()
	adapterStart := time.Now()
	firstEvent := true
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "openai", "stream", true)

	streamErr := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		if firstEvent {
			telemetry.Event(c, "adapter.first_event", "protocol", "openai", "ttfb_ms", time.Since(adapterStart).Milliseconds())
			firstEvent = false
		}
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			if r.Chunk.Text != "" {
				write(protocol.NewOpenAIStreamChunk(messageID, model, created, r.Chunk.Text, nil))
			}

		case *pb.UniversalResponse_ToolCall:
			// 工具调用：先发带 id/name 的 delta，再发 arguments delta
			toolIdx++
			write(protocol.FormatOpenAISSE(&protocol.OpenAIStreamResponse{
				ID: messageID, Object: "chat.completion.chunk", Created: created, Model: model,
				Choices: []protocol.StreamChoice{{
					Index: 0,
					Delta: protocol.DeltaMessage{
						ToolCalls: []protocol.ToolCall{{
							Index: intPtr(toolIdx), ID: r.ToolCall.Id, Type: "function",
							Function: map[string]interface{}{"name": r.ToolCall.Name, "arguments": ""},
						}},
					},
				}},
			}))
			if r.ToolCall.Arguments != "" {
				write(protocol.FormatOpenAISSE(&protocol.OpenAIStreamResponse{
					ID: messageID, Object: "chat.completion.chunk", Created: created, Model: model,
					Choices: []protocol.StreamChoice{{
						Index: 0,
						Delta: protocol.DeltaMessage{
							ToolCalls: []protocol.ToolCall{{
								Index:    intPtr(toolIdx),
								Function: map[string]interface{}{"arguments": r.ToolCall.Arguments},
							}},
						},
					}},
				}))
			}
			finishReason = "tool_calls"

		case *pb.UniversalResponse_Completion:
			finishReason = mapOpenAIFinishReason(r.Completion.FinishReason)

		case *pb.UniversalResponse_Error:
			// OpenAI 流式错误：发一个 error 数据块
			errObj := map[string]interface{}{"error": map[string]interface{}{"message": r.Error.Message, "type": "api_error"}}
			if b, err := json.Marshal(errObj); err == nil {
				write("data: " + string(b) + "\n\n")
			}
			return fmt.Errorf("upstream error: %s", r.Error.Message)
		}
		return nil
	})
	if streamErr != nil {
		span.EndError(streamErr)
	} else {
		span.End()
	}

	if finishReason == "" {
		finishReason = "stop"
	}
	write(protocol.NewOpenAIStreamChunk(messageID, model, created, "", &finishReason))
	write(protocol.NewOpenAIStreamDone())

	_ = streamErr
}

// mapOpenAIFinishReason 把 universal finish_reason 映射为 OpenAI finish_reason。
func mapOpenAIFinishReason(finishReason string) string {
	switch finishReason {
	case "", "stop", "end_turn":
		return "stop"
	case "length", "max_tokens":
		return "length"
	case "tool_calls", "tool_use":
		return "tool_calls"
	case "content_filter":
		return "content_filter"
	default:
		return finishReason
	}
}

// finalizeOpenAIFinish 规整非流式响应的 finish_reason（converter 当前直接透传，需校正）。
func finalizeOpenAIFinish(resp *protocol.OpenAIResponse) {
	if len(resp.Choices) == 0 {
		return
	}
	fr := mapOpenAIFinishReason(resp.Choices[0].FinishReason)
	if fr == "" {
		fr = "stop"
	}
	resp.Choices[0].FinishReason = fr
}

// intPtr 返回 int 指针（流式 tool_calls 的 index 字段需要）。
func intPtr(i int) *int {
	return &i
}
