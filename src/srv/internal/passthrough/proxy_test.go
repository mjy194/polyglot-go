package passthrough

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"polyglot/internal/config"
)

func TestProxyForwardsOpenAIRequestWithBearerKey(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	proxy := New(config.PassthroughConfig{
		Enabled: true,
		Upstreams: map[string]config.UpstreamConfig{
			ProtocolOpenAI: {BaseURL: upstream.URL + "/v1", APIKey: "sk-test"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?trace=1", strings.NewReader(`{"model":"gpt"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	if err := proxy.ServeHTTP(w, req, ProtocolOpenAI); err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	if w.Code != http.StatusAccepted || strings.TrimSpace(w.Body.String()) != `{"ok":true}` {
		t.Fatalf("unexpected response status=%d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/chat/completions?trace=1" {
		t.Fatalf("path = %q, want /v1/chat/completions?trace=1", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody != `{"model":"gpt"}` {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestProxyNormalizesAPIPrefixForGeminiAndSetsGoogleKey(t *testing.T) {
	var gotPath, gotKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-goog-api-key")
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer upstream.Close()

	proxy := New(config.PassthroughConfig{
		Enabled: true,
		Upstreams: map[string]config.UpstreamConfig{
			ProtocolGemini: {URL: upstream.URL, APIKey: "gem-key"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1beta/models/gemini-pro:generateContent", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	if err := proxy.ServeHTTP(w, req, ProtocolGemini); err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	if gotPath != "/v1beta/models/gemini-pro:generateContent" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotKey != "gem-key" {
		t.Fatalf("x-goog-api-key = %q", gotKey)
	}
}

func TestProxyResponsesFallsBackToOpenAIUpstream(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"resp_1"}`))
	}))
	defer upstream.Close()

	proxy := New(config.PassthroughConfig{
		Enabled: true,
		Upstreams: map[string]config.UpstreamConfig{
			ProtocolOpenAI: {URL: upstream.URL, APIKey: "sk-test"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	if err := proxy.ServeHTTP(w, req, ProtocolResponses); err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	if gotPath != "/v1/responses" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestProxyAnthropicSetsAPIKeyAndDefaultVersion(t *testing.T) {
	var gotKey, gotVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		_, _ = w.Write([]byte(`{"type":"message"}`))
	}))
	defer upstream.Close()

	proxy := New(config.PassthroughConfig{
		Enabled: true,
		Upstreams: map[string]config.UpstreamConfig{
			ProtocolAnthropic: {URL: upstream.URL, APIKey: "anth-key"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	if err := proxy.ServeHTTP(w, req, ProtocolAnthropic); err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	if gotKey != "anth-key" {
		t.Fatalf("x-api-key = %q", gotKey)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q", gotVersion)
	}
}
