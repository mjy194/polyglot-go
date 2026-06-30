package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"polyglot/internal/adapter"
	pb "polyglot/proto/adapter"

	"github.com/gin-gonic/gin"
)

type protocolMatrixCase struct {
	name        string
	path        string
	body        string
	handler     gin.HandlerFunc
	wantStatus  int
	wantMarkers []string
}

func TestProtocolNonStreamCorrectnessMatrix(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("matrix text", 0, true),
		completionMsg("stop", 4, 6),
	}}
	resolve := func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true }

	runProtocolMatrix(t, []protocolMatrixCase{
		{
			name:       "anthropic messages",
			path:       "/v1/messages",
			body:       `{"model":"claude-3-5-sonnet","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
			handler:    AnthropicMessages(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				`"type":"message"`,
				`"model":"claude-3-5-sonnet"`,
				`"text":"matrix text"`,
				`"stop_reason":"end_turn"`,
				`"input_tokens":4`,
				`"output_tokens":6`,
			},
		},
		{
			name:       "openai chat completions",
			path:       "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			handler:    OpenAIChatCompletions(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				`"object":"chat.completion"`,
				`"model":"gpt-4o"`,
				`"content":"matrix text"`,
				`"finish_reason":"stop"`,
				`"prompt_tokens":4`,
				`"completion_tokens":6`,
			},
		},
		{
			name:       "openai responses",
			path:       "/v1/responses",
			body:       `{"model":"gpt-4o","input":"hi"}`,
			handler:    Responses(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				`"object":"response"`,
				`"model":"gpt-4o"`,
				`"status":"completed"`,
				`"type":"output_text"`,
				`"text":"matrix text"`,
				`"total_tokens":10`,
			},
		},
		{
			name:       "gemini generate content",
			path:       "/v1beta/models/gemini-1.5-pro:generateContent",
			body:       `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`,
			handler:    GeminiGenerateContent(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				`"candidates"`,
				`"role":"model"`,
				`"text":"matrix text"`,
				`"finishReason":"STOP"`,
				`"promptTokenCount":4`,
				`"candidatesTokenCount":6`,
			},
		},
	})
}

func TestProtocolStreamCorrectnessMatrix(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("matrix", 0, false),
		chunkMsg(" stream", 0, true),
		completionMsg("max_tokens", 3, 7),
	}}
	resolve := func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true }

	runProtocolMatrix(t, []protocolMatrixCase{
		{
			name:       "anthropic stream",
			path:       "/v1/messages",
			body:       `{"model":"claude-3-5-sonnet","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`,
			handler:    AnthropicMessages(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				"event: message_start",
				"event: content_block_delta",
				`"text":"matrix"`,
				`"text":" stream"`,
				`"stop_reason":"max_tokens"`,
				"event: message_stop",
			},
		},
		{
			name:       "openai stream",
			path:       "/v1/chat/completions",
			body:       `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`,
			handler:    OpenAIChatCompletions(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				"data: ",
				`"role":"assistant"`,
				`"content":"matrix"`,
				`"content":" stream"`,
				`"finish_reason":"length"`,
				"data: [DONE]",
			},
		},
		{
			name:       "responses stream",
			path:       "/v1/responses",
			body:       `{"model":"gpt-4o","input":"hi","stream":true}`,
			handler:    Responses(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				"event: response.created",
				"event: response.output_text.delta",
				`"delta":"matrix"`,
				`"delta":" stream"`,
				"event: response.completed",
				`"total_tokens":10`,
			},
		},
		{
			name:       "gemini stream",
			path:       "/v1beta/models/gemini-1.5-pro:generateContent?alt=sse",
			body:       `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`,
			handler:    GeminiGenerateContent(resolve),
			wantStatus: http.StatusOK,
			wantMarkers: []string{
				"data: ",
				`"text":"matrix"`,
				`"text":" stream"`,
				`"finishReason":"MAX_TOKENS"`,
				`"totalTokenCount":10`,
			},
		},
	})
}

func TestProtocolValidationCorrectnessMatrix(t *testing.T) {
	resolve := func(c *gin.Context) (adapter.StreamProcessor, bool) { return fakeProcessor{}, true }

	runProtocolMatrix(t, []protocolMatrixCase{
		{
			name:       "anthropic missing max_tokens",
			path:       "/v1/messages",
			body:       `{"model":"claude","messages":[{"role":"user","content":"hi"}]}`,
			handler:    AnthropicMessages(resolve),
			wantStatus: http.StatusBadRequest,
			wantMarkers: []string{
				`"type":"invalid_request_error"`,
				"max_tokens",
			},
		},
		{
			name:       "openai missing model",
			path:       "/v1/chat/completions",
			body:       `{"messages":[{"role":"user","content":"hi"}]}`,
			handler:    OpenAIChatCompletions(resolve),
			wantStatus: http.StatusBadRequest,
			wantMarkers: []string{
				`"type":"invalid_request_error"`,
				"model is required",
			},
		},
		{
			name:       "responses missing input",
			path:       "/v1/responses",
			body:       `{"model":"gpt-4o"}`,
			handler:    Responses(resolve),
			wantStatus: http.StatusBadRequest,
			wantMarkers: []string{
				`"type":"invalid_request_error"`,
				"input is required",
			},
		},
		{
			name:       "gemini missing contents",
			path:       "/v1beta/models/gemini-pro:generateContent",
			body:       `{}`,
			handler:    GeminiGenerateContent(resolve),
			wantStatus: http.StatusBadRequest,
			wantMarkers: []string{
				`"status":"INVALID_ARGUMENT"`,
				"contents is required",
			},
		},
	})
}

func TestProtocolAdapterErrorCorrectnessMatrix(t *testing.T) {
	fp := fakeProcessor{err: errors.New("adapter failed")}
	resolve := func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true }

	runProtocolMatrix(t, []protocolMatrixCase{
		{
			name:       "anthropic adapter error",
			path:       "/v1/messages",
			body:       `{"model":"claude","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
			handler:    AnthropicMessages(resolve),
			wantStatus: http.StatusBadGateway,
			wantMarkers: []string{
				`"type":"api_error"`,
				"adapter failed",
			},
		},
		{
			name:       "openai adapter error",
			path:       "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			handler:    OpenAIChatCompletions(resolve),
			wantStatus: http.StatusBadGateway,
			wantMarkers: []string{
				`"type":"api_error"`,
				"adapter failed",
			},
		},
		{
			name:       "responses adapter error",
			path:       "/v1/responses",
			body:       `{"model":"gpt-4o","input":"hi"}`,
			handler:    Responses(resolve),
			wantStatus: http.StatusBadGateway,
			wantMarkers: []string{
				`"type":"server_error"`,
				"adapter failed",
			},
		},
		{
			name:       "gemini adapter error",
			path:       "/v1beta/models/gemini-pro:generateContent",
			body:       `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`,
			handler:    GeminiGenerateContent(resolve),
			wantStatus: http.StatusBadGateway,
			wantMarkers: []string{
				`"status":"UNAVAILABLE"`,
				"adapter failed",
			},
		},
	})
}

func TestProtocolNoAdapterCorrectnessMatrix(t *testing.T) {
	resolve := func(c *gin.Context) (adapter.StreamProcessor, bool) { return nil, false }

	runProtocolMatrix(t, []protocolMatrixCase{
		{
			name:       "anthropic no adapter",
			path:       "/v1/messages",
			body:       `{"model":"claude","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
			handler:    AnthropicMessages(resolve),
			wantStatus: http.StatusServiceUnavailable,
			wantMarkers: []string{
				`"type":"api_error"`,
				"no adapter registered",
			},
		},
		{
			name:       "openai no adapter",
			path:       "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			handler:    OpenAIChatCompletions(resolve),
			wantStatus: http.StatusServiceUnavailable,
			wantMarkers: []string{
				`"type":"api_error"`,
				"no adapter registered",
			},
		},
		{
			name:       "responses no adapter",
			path:       "/v1/responses",
			body:       `{"model":"gpt-4o","input":"hi"}`,
			handler:    Responses(resolve),
			wantStatus: http.StatusServiceUnavailable,
			wantMarkers: []string{
				`"type":"server_error"`,
				"no adapter registered",
			},
		},
		{
			name:       "gemini no adapter",
			path:       "/v1beta/models/gemini-pro:generateContent",
			body:       `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`,
			handler:    GeminiGenerateContent(resolve),
			wantStatus: http.StatusServiceUnavailable,
			wantMarkers: []string{
				`"status":"UNAVAILABLE"`,
				"no adapter registered",
			},
		},
	})
}

func runProtocolMatrix(t *testing.T, cases []protocolMatrixCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/*path", tc.handler)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			body := w.Body.String()
			for _, marker := range tc.wantMarkers {
				if !strings.Contains(body, marker) {
					t.Fatalf("missing marker %q\n--- body ---\n%s", marker, body)
				}
			}
		})
	}
}
