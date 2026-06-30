package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"polyglot/internal/adapter"
	"polyglot/internal/converter"
	"polyglot/internal/protocol"
	"polyglot/internal/telemetry"
	pb "polyglot/proto/adapter"

	"github.com/gin-gonic/gin"
)

// GeminiGenerateContent Gemini generateContent API handler。
// 链路：校验 → GeminiToUniversal → adapter.ProcessStream → UniversalToGemini。
// model 取自 URL 路径 /api/v1beta/{model}[:generateContent]；流式由 ?alt=sse 决定。
func GeminiGenerateContent(resolve StreamProcessorResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 解析请求
		var req protocol.GeminiRequest
		parseSpan := telemetry.Start(c, "protocol.parse", "protocol", "gemini")
		if err := c.ShouldBindJSON(&req); err != nil {
			parseSpan.EndError(err)
			geminiError(c, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		parseSpan.End()

		// 2. 校验
		validateSpan := telemetry.Start(c, "protocol.validate", "protocol", "gemini")
		if err := req.Validate(); err != nil {
			validateSpan.EndError(err)
			geminiError(c, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		validateSpan.End()

		// 3. 解析 adapter（动态寻址）
		processor, ok := resolve(c)
		if !ok {
			geminiError(c, http.StatusServiceUnavailable, "UNAVAILABLE",
				"no adapter registered for this backend; service unavailable")
			return
		}

		// 4. Gemini → Universal
		convertSpan := telemetry.Start(c, "universal.convert_request", "protocol", "gemini")
		univReq, err := converter.GeminiToUniversal(&req)
		if err != nil {
			convertSpan.EndError(err)
			geminiError(c, http.StatusBadRequest, "INVALID_ARGUMENT",
				fmt.Sprintf("failed to convert request: %v", err))
			return
		}
		// converter 把 model 写死成 "gemini-pro"；用 URL 路径里的真实 model 覆盖
		univReq.Model = geminiModelFromPath(c.Param("model"))

		isStream := c.Query("alt") == "sse"
		if univReq.Config != nil {
			univReq.Config.Stream = isStream
		}
		convertSpan.End("model", univReq.Model, "messages", len(univReq.Messages), "tools", len(univReq.Tools), "stream", isStream)
		telemetry.SetField(c, "model", univReq.Model)
		telemetry.SetField(c, "stream", isStream)

		// 5. 处理 + 响应
		if isStream {
			streamGeminiResponse(c, processor, univReq)
		} else {
			handleGeminiNonStream(c, processor, univReq)
		}
	}
}

// geminiModelFromPath 从路由参数解析 model，去掉 ":generateContent" 之类的后缀。
func geminiModelFromPath(p string) string {
	if p == "" {
		return "gemini-pro"
	}
	if i := strings.Index(p, ":"); i > 0 {
		return p[:i]
	}
	return p
}

// handleGeminiNonStream 非流式：收集全部 universal 响应后一次性转换并返回。
func handleGeminiNonStream(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest) {
	ctx := c.Request.Context()

	var responses []*pb.UniversalResponse
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "gemini", "stream", false)
	err := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		responses = append(responses, resp)
		return nil
	})
	if err != nil {
		span.EndError(err, "responses", len(responses))
		geminiError(c, http.StatusBadGateway, "UNAVAILABLE",
			fmt.Sprintf("upstream adapter error: %v", err))
		return
	}
	span.End("responses", len(responses))

	for _, r := range responses {
		if e := r.GetError(); e != nil {
			geminiError(c, http.StatusBadGateway, "UNAVAILABLE", e.Message)
			return
		}
	}

	convertSpan := telemetry.Start(c, "universal.convert_response", "protocol", "gemini", "responses", len(responses))
	resp, err := converter.UniversalToGemini(responses)
	if err != nil {
		convertSpan.EndError(err)
		geminiError(c, http.StatusInternalServerError, "INTERNAL",
			fmt.Sprintf("failed to convert response: %v", err))
		return
	}
	convertSpan.End("candidates", len(resp.Candidates))
	finalizeGeminiFinish(resp)

	c.JSON(http.StatusOK, resp)
}

// streamGeminiResponse 流式：把 universal 响应实时转成 Gemini SSE chunk。
func streamGeminiResponse(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		geminiError(c, http.StatusInternalServerError, "INTERNAL", "streaming not supported")
		return
	}
	write := func(s string) {
		_, _ = c.Writer.WriteString(s)
		flusher.Flush()
	}

	var finishReason string
	var usage *protocol.GeminiUsageMetadata
	ctx := c.Request.Context()
	adapterStart := time.Now()
	firstEvent := true
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "gemini", "stream", true)

	streamErr := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		if firstEvent {
			telemetry.Event(c, "adapter.first_event", "protocol", "gemini", "ttfb_ms", time.Since(adapterStart).Milliseconds())
			firstEvent = false
		}
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			if r.Chunk.Text != "" {
				write(protocol.NewGeminiStreamChunk(r.Chunk.Text, ""))
			}

		case *pb.UniversalResponse_ToolCall:
			args := map[string]interface{}{}
			if r.ToolCall.Arguments != "" {
				_ = json.Unmarshal([]byte(r.ToolCall.Arguments), &args)
			}
			write(protocol.NewGeminiFunctionCallChunk(r.ToolCall.Name, args, ""))

		case *pb.UniversalResponse_Completion:
			finishReason = mapGeminiFinishReason(r.Completion.FinishReason)
			in := int(r.Completion.InputTokens)
			out := int(r.Completion.OutputTokens)
			usage = &protocol.GeminiUsageMetadata{
				PromptTokenCount:     in,
				CandidatesTokenCount: out,
				TotalTokenCount:      in + out,
			}

		case *pb.UniversalResponse_Error:
			errObj := map[string]interface{}{"error": map[string]interface{}{
				"code": 500, "message": r.Error.Message, "status": "INTERNAL",
			}}
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

	if streamErr != nil {
		return
	}

	if finishReason == "" {
		finishReason = "STOP"
	}
	// 收尾 chunk：带 finishReason（及 usage，若有）
	finalChunk := &protocol.GeminiStreamResponse{
		Candidates: []protocol.GeminiCandidate{{
			Content: protocol.GeminiContent{
				Role:  "model",
				Parts: []protocol.GeminiPart{{Text: " "}},
			},
			FinishReason: finishReason,
			Index:        0,
		}},
		UsageMetadata: usage,
	}
	write(protocol.FormatGeminiSSE(finalChunk))

	_ = streamErr
}

// mapGeminiFinishReason 把 universal finish_reason 映射为 Gemini finishReason。
func mapGeminiFinishReason(finishReason string) string {
	switch finishReason {
	case "", "stop", "end_turn":
		return "STOP"
	case "length", "max_tokens":
		return "MAX_TOKENS"
	case "safety", "content_filter":
		return "SAFETY"
	case "tool_calls", "tool_use":
		// Gemini 没有专门的工具调用 finishReason；工具调用本身即终止，按 STOP
		return "STOP"
	default:
		return finishReason
	}
}

// finalizeGeminiFinish 规整非流式响应的 finishReason。
func finalizeGeminiFinish(resp *protocol.GeminiResponse) {
	if len(resp.Candidates) == 0 {
		return
	}
	fr := mapGeminiFinishReason(resp.Candidates[0].FinishReason)
	if fr == "" {
		fr = "STOP"
	}
	resp.Candidates[0].FinishReason = fr
}

// geminiError 返回 Gemini 风格的错误 JSON。
func geminiError(c *gin.Context, status int, statusStr, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    status,
			"message": message,
			"status":  statusStr,
		},
	})
}
