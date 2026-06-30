package handler

import (
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
	"github.com/google/uuid"
)

// Responses OpenAI Responses API handler（/v1/responses，codex CLI 走此路径）。
// 链路：校验 → ResponsesToUniversal → adapter.ProcessStream → UniversalToResponses。
func Responses(resolve StreamProcessorResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 解析请求
		var req protocol.ResponseRequest
		parseSpan := telemetry.Start(c, "protocol.parse", "protocol", "responses")
		if err := c.ShouldBindJSON(&req); err != nil {
			parseSpan.EndError(err)
			responsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		parseSpan.End("model", req.Model, "stream", req.Stream)
		telemetry.SetField(c, "model", req.Model)
		telemetry.SetField(c, "stream", req.Stream)

		// 2. 校验
		validateSpan := telemetry.Start(c, "protocol.validate", "protocol", "responses")
		if err := req.Validate(); err != nil {
			validateSpan.EndError(err)
			responsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		validateSpan.End()

		// 3. 解析 adapter
		processor, ok := resolve(c)
		if !ok {
			responsesError(c, http.StatusServiceUnavailable, "server_error",
				"no adapter registered for this backend; service unavailable")
			return
		}

		// 4. Responses → Universal
		convertSpan := telemetry.Start(c, "universal.convert_request", "protocol", "responses")
		univReq, err := converter.ResponsesToUniversal(&req)
		if err != nil {
			convertSpan.EndError(err)
			responsesError(c, http.StatusBadRequest, "invalid_request_error",
				fmt.Sprintf("failed to convert request: %v", err))
			return
		}
		convertSpan.End("messages", len(univReq.Messages), "tools", len(univReq.Tools))

		responseID := "resp_" + uuid.New().String()

		// 5. 处理 + 响应
		if req.Stream {
			streamResponses(c, processor, univReq, responseID, req.Model)
		} else {
			handleResponsesNonStream(c, processor, univReq, responseID, req.Model)
		}
	}
}

// handleResponsesNonStream 非流式
func handleResponsesNonStream(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest, responseID, model string) {
	ctx := c.Request.Context()

	var responses []*pb.UniversalResponse
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "responses", "stream", false)
	err := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		responses = append(responses, resp)
		return nil
	})
	if err != nil {
		span.EndError(err, "responses", len(responses))
		responsesError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("upstream adapter error: %v", err))
		return
	}
	span.End("responses", len(responses))
	for _, r := range responses {
		if e := r.GetError(); e != nil {
			responsesError(c, http.StatusBadGateway, "server_error", e.Message)
			return
		}
	}

	convertSpan := telemetry.Start(c, "universal.convert_response", "protocol", "responses", "responses", len(responses))
	resp, err := converter.UniversalToResponses(responseID, model, responses)
	if err != nil {
		convertSpan.EndError(err)
		responsesError(c, http.StatusInternalServerError, "server_error",
			fmt.Sprintf("failed to convert response: %v", err))
		return
	}
	convertSpan.End("output_items", len(resp.Output))
	c.JSON(http.StatusOK, resp)
}

// streamResponses 流式：按 Responses API 规范发送事件序列：
//
//	response.created → output_item.added(message) → content_part.added(output_text)
//	→ output_text.delta(*) → output_text.done → content_part.done → output_item.done
//	→ response.completed
func streamResponses(c *gin.Context, processor adapter.StreamProcessor, univReq *pb.UniversalRequest, responseID, model string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(gin.ResponseWriter)
	if !ok {
		responsesError(c, http.StatusInternalServerError, "server_error", "streaming not supported")
		return
	}
	write := func(s string) {
		_, _ = c.Writer.WriteString(s)
		flusher.Flush()
	}

	msgID := "msg_" + uuid.New().String()
	funcID := "fc_" + uuid.New().String()
	created := float64(time.Now().Unix())
	seq := 0
	nextSeq := func() int { s := seq; seq++; return s }

	emptyResp := &protocol.ResponseResponse{
		ID: responseID, Object: "response", Model: model, CreatedAt: created,
		Status: "in_progress", Output: []protocol.ResponseOutputItem{},
	}

	// 1. response.created
	write(protocol.FormatResponseSSE("response.created", gin.H{
		"type": "response.created", "sequence_number": nextSeq(), "response": emptyResp,
	}))

	// 2. output_item.added（message，content 暂空）
	msgItem := protocol.ResponseOutputItem{
		Type: "message", ID: msgID, Role: "assistant", Status: "in_progress",
		Content: []protocol.ResponseOutputContent{},
	}
	write(protocol.FormatResponseSSE("response.output_item.added", gin.H{
		"type": "response.output_item.added", "sequence_number": nextSeq(),
		"output_index": 0, "item": msgItem,
	}))

	// 3. content_part.added（output_text）
	write(protocol.FormatResponseSSE("response.content_part.added", gin.H{
		"type": "response.content_part.added", "sequence_number": nextSeq(),
		"item_id": msgID, "output_index": 0, "content_index": 0,
		"part": gin.H{"type": "output_text", "text": "", "annotations": []interface{}{}},
	}))

	// 4. 逐 chunk 发 output_text.delta
	var fullText strings.Builder
	var inTokens, outTokens int
	ctx := c.Request.Context()
	adapterStart := time.Now()
	firstEvent := true
	span := telemetry.Start(c, "adapter.universal_stream", "protocol", "responses", "stream", true)

	streamErr := processor.ProcessStream(ctx, univReq, func(resp *pb.UniversalResponse) error {
		if firstEvent {
			telemetry.Event(c, "adapter.first_event", "protocol", "responses", "ttfb_ms", time.Since(adapterStart).Milliseconds())
			firstEvent = false
		}
		switch v := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			if v.Chunk.Text != "" {
				fullText.WriteString(v.Chunk.Text)
				write(protocol.FormatResponseSSE("response.output_text.delta", gin.H{
					"type": "response.output_text.delta", "sequence_number": nextSeq(),
					"item_id": msgID, "output_index": 0, "content_index": 0, "delta": v.Chunk.Text,
				}))
			}
		case *pb.UniversalResponse_ToolCall:
			write(protocol.FormatResponseSSE("response.output_item.added", gin.H{
				"type": "response.output_item.added", "sequence_number": nextSeq(),
				"output_index": 1, "item": protocol.ResponseOutputItem{
					Type:      "function_call",
					ID:        funcID,
					CallID:    v.ToolCall.Id,
					Name:      v.ToolCall.Name,
					Arguments: "",
					Status:    "completed",
					Content:   []protocol.ResponseOutputContent{},
				},
			}))
			write(protocol.FormatResponseSSE("response.content_part.added", gin.H{
				"type": "response.content_part.added", "sequence_number": nextSeq(),
				"item_id": funcID, "output_index": 1, "content_index": 0,
				"part": gin.H{"type": "function_call", "name": v.ToolCall.Name, "arguments": ""},
			}))
			if v.ToolCall.Arguments != "" {
				write(protocol.FormatResponseSSE("response.function_call_arguments.delta", gin.H{
					"type": "response.function_call_arguments.delta", "sequence_number": nextSeq(),
					"item_id": funcID, "output_index": 1, "content_index": 0, "delta": v.ToolCall.Arguments,
				}))
			}
			write(protocol.FormatResponseSSE("response.content_part.done", gin.H{
				"type": "response.content_part.done", "sequence_number": nextSeq(),
				"item_id": funcID, "output_index": 1, "content_index": 0,
				"part": gin.H{"type": "function_call", "name": v.ToolCall.Name, "arguments": v.ToolCall.Arguments},
			}))
			write(protocol.FormatResponseSSE("response.output_item.done", gin.H{
				"type": "response.output_item.done", "sequence_number": nextSeq(),
				"output_index": 1, "item": protocol.ResponseOutputItem{
					Type:      "function_call",
					ID:        funcID,
					CallID:    v.ToolCall.Id,
					Name:      v.ToolCall.Name,
					Arguments: v.ToolCall.Arguments,
					Status:    "completed",
					Content:   []protocol.ResponseOutputContent{},
				},
			}))
		case *pb.UniversalResponse_Completion:
			inTokens = int(v.Completion.InputTokens)
			outTokens = int(v.Completion.OutputTokens)
		case *pb.UniversalResponse_Error:
			write(protocol.FormatResponseSSE("error", gin.H{"code": "server_error", "message": v.Error.Message}))
			return fmt.Errorf("upstream error: %s", v.Error.Message)
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

	text := fullText.String()

	// 5. output_text.done
	write(protocol.FormatResponseSSE("response.output_text.done", gin.H{
		"type": "response.output_text.done", "sequence_number": nextSeq(),
		"item_id": msgID, "output_index": 0, "content_index": 0, "text": text,
	}))
	// 6. content_part.done
	write(protocol.FormatResponseSSE("response.content_part.done", gin.H{
		"type": "response.content_part.done", "sequence_number": nextSeq(),
		"item_id": msgID, "output_index": 0, "content_index": 0,
		"part": gin.H{"type": "output_text", "text": text, "annotations": []interface{}{}},
	}))
	// 7. output_item.done（完整 message）
	doneItem := protocol.ResponseOutputItem{
		Type: "message", ID: msgID, Role: "assistant", Status: "completed",
		Content: []protocol.ResponseOutputContent{{Type: "output_text", Text: text, Annotations: []interface{}{}}},
	}
	write(protocol.FormatResponseSSE("response.output_item.done", gin.H{
		"type": "response.output_item.done", "sequence_number": nextSeq(),
		"output_index": 0, "item": doneItem,
	}))
	// 8. response.completed
	completedResp := &protocol.ResponseResponse{
		ID: responseID, Object: "response", Model: model, CreatedAt: created,
		Status: "completed", Output: []protocol.ResponseOutputItem{doneItem},
		Usage: protocol.ResponseUsage{InputTokens: inTokens, OutputTokens: outTokens, TotalTokens: inTokens + outTokens},
	}
	write(protocol.FormatResponseSSE("response.completed", gin.H{
		"type": "response.completed", "sequence_number": nextSeq(), "response": completedResp,
	}))

	_ = streamErr
}

// responsesError 返回 OpenAI 风格的错误 JSON。
func responsesError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}
