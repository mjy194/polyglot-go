package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"polyglot/internal/protocol"
	pb "polyglot/proto/adapter"

	"github.com/google/uuid"
)

// ============================================================================
// Responses → Universal
// ============================================================================

// ResponsesToUniversal 将 Responses API 请求转换为 universal 格式。
// input 可为 string 或 []input_item（message / function_call / function_call_output）；
// instructions 可为 string 或 []text 块，映射到 universal 的 system。
func ResponsesToUniversal(req *protocol.ResponseRequest) (*pb.UniversalRequest, error) {
	var messages []*pb.Message
	system := responsesDecodeText(req.Instructions)

	// input
	if len(req.Input) > 0 && !isResponsesNullOrBlank(req.Input) {
		if s, ok := responsesUnwrapString(req.Input); ok {
			messages = append(messages, textMessage(pb.Message_USER, s))
		} else {
			var items []protocol.ResponseInputItem
			if err := json.Unmarshal(req.Input, &items); err != nil {
				return nil, fmt.Errorf("invalid input: %w", err)
			}
			for _, it := range items {
				if msg := responsesItemToMessage(it); msg != nil {
					if msg.Role == pb.Message_SYSTEM {
						system = appendSystemText(system, messageText(msg))
						continue
					}
					messages = append(messages, msg)
				}
			}
		}
	}

	// tools
	var tools []*pb.Tool
	for _, t := range req.Tools {
		params, _ := json.Marshal(t.Parameters)
		tools = append(tools, &pb.Tool{Name: t.Name, Description: t.Description, Parameters: string(params)})
	}

	cfg := &pb.GenerationConfig{
		MaxTokens:   int32(req.MaxOutputTokens),
		Temperature: responsesDerefFloat(req.Temperature),
		TopP:        responsesDerefFloat(req.TopP),
		Stream:      req.Stream,
	}

	return &pb.UniversalRequest{
		RequestId: uuid.New().String(),
		Model:     req.Model,
		Messages:  messages,
		System:    system,
		Tools:     tools,
		Config:    cfg,
	}, nil
}

// ============================================================================
// Universal → Responses
// ============================================================================

// UniversalToResponses 将 universal 响应转换为 Responses API 响应（非流式）。
// 文本 chunk 合并进单个 message/output_text 项。
func UniversalToResponses(responseID, model string, responses []*pb.UniversalResponse) (*protocol.ResponseResponse, error) {
	var textBuf strings.Builder
	var output []protocol.ResponseOutputItem
	var inTokens, outTokens int
	status := "completed"

	for _, resp := range responses {
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			textBuf.WriteString(r.Chunk.Text)
		case *pb.UniversalResponse_ToolCall:
			output = append(output, protocol.ResponseOutputItem{
				Type:      "function_call",
				ID:        "fc_" + r.ToolCall.Id,
				CallID:    r.ToolCall.Id,
				Name:      r.ToolCall.Name,
				Arguments: r.ToolCall.Arguments,
				Status:    "completed",
				Content:   []protocol.ResponseOutputContent{},
			})
		case *pb.UniversalResponse_Completion:
			inTokens = int(r.Completion.InputTokens)
			outTokens = int(r.Completion.OutputTokens)
		case *pb.UniversalResponse_Error:
			status = "failed"
		}
	}

	if textBuf.Len() > 0 || len(output) == 0 {
		outItem := protocol.ResponseOutputItem{
			Type:    "message",
			ID:      "msg_" + responseID,
			Role:    "assistant",
			Status:  "completed",
			Content: []protocol.ResponseOutputContent{},
		}
		if textBuf.Len() > 0 {
			outItem.Content = []protocol.ResponseOutputContent{
				{Type: "output_text", Text: textBuf.String(), Annotations: []interface{}{}},
			}
		}
		output = append([]protocol.ResponseOutputItem{outItem}, output...)
	}

	return &protocol.ResponseResponse{
		ID:     responseID,
		Object: "response",
		Model:  model,
		Status: status,
		Output: output,
		Usage: protocol.ResponseUsage{
			InputTokens:  inTokens,
			OutputTokens: outTokens,
			TotalTokens:  inTokens + outTokens,
		},
	}, nil
}

// ============================================================================
// Helpers
// ============================================================================

func textMessage(role pb.Message_Role, text string) *pb.Message {
	return &pb.Message{
		Role: role,
		Content: []*pb.ContentPart{
			{Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: text}}},
		},
	}
}

func responsesItemToMessage(it protocol.ResponseInputItem) *pb.Message {
	if it.Type == "function_call" {
		if it.CallID == "" || it.Name == "" {
			return nil
		}
		return &pb.Message{
			Role: pb.Message_ASSISTANT,
			Content: []*pb.ContentPart{{
				Part: &pb.ContentPart_ToolCall{
					ToolCall: &pb.ToolCallPart{
						Id:        it.CallID,
						Name:      it.Name,
						Arguments: normalizeJSONText(it.Arguments),
					},
				},
			}},
		}
	}

	if it.Type == "function_call_output" {
		output, ok := responsesOutputString(it.Output)
		if it.CallID == "" || !ok {
			return nil
		}
		return &pb.Message{
			Role: pb.Message_TOOL,
			Content: []*pb.ContentPart{{
				Part: &pb.ContentPart_ToolResult{
					ToolResult: &pb.ToolResultPart{
						ToolCallId: it.CallID,
						Result:     output,
					},
				},
			}},
		}
	}

	role := pb.Message_USER
	switch it.Role {
	case "assistant":
		role = pb.Message_ASSISTANT
	case "system", "developer":
		role = pb.Message_SYSTEM
	}

	var parts []*pb.ContentPart
	if len(it.Content) > 0 && !isResponsesNullOrBlank(it.Content) {
		if s, ok := responsesUnwrapString(it.Content); ok {
			parts = append(parts, textPart(s))
		} else {
			var cps []protocol.ResponseContentPart
			if err := json.Unmarshal(it.Content, &cps); err == nil {
				for _, c := range cps {
					switch c.Type {
					case "input_text", "output_text", "text", "":
						if c.Text == "" {
							continue
						}
						parts = append(parts, textPart(c.Text))
					case "input_image":
						if url, detail := c.ImageURLValue(); url != "" {
							parts = append(parts, &pb.ContentPart{
								Part: &pb.ContentPart_Image{
									Image: &pb.ImagePart{
										Source: &pb.ImagePart_Url{Url: url},
										Detail: detail,
									},
								},
							})
						}
					}
				}
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return &pb.Message{Role: role, Content: parts}
}

func textPart(s string) *pb.ContentPart {
	return &pb.ContentPart{Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: s}}}
}

func messageText(msg *pb.Message) string {
	var parts []string
	for _, part := range msg.GetContent() {
		if text := part.GetText(); text != nil && text.Text != "" {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// responsesUnwrapString 若 RawMessage 是 JSON 字符串则返回其值。
func responsesUnwrapString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return "", false
}

// responsesDecodeText 解码 string 或 []{type:text,text} 形态的文本字段。
func responsesDecodeText(raw json.RawMessage) string {
	if len(raw) == 0 || isResponsesNullOrBlank(raw) {
		return ""
	}
	if s, ok := responsesUnwrapString(raw); ok {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			sb.WriteString(b.Text)
		}
		return sb.String()
	}
	return ""
}

func responsesOutputString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || isResponsesNullOrBlank(raw) {
		return "", false
	}
	if s, ok := responsesUnwrapString(raw); ok {
		return s, true
	}
	return string(raw), true
}

func normalizeJSONText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}

func responsesDerefFloat(p *float64) float64 {
	if p != nil {
		return *p
	}
	return 0
}

func isResponsesNullOrBlank(raw json.RawMessage) bool {
	s := string(raw)
	return s == "null" || s == "\"\"" || s == "[]"
}
