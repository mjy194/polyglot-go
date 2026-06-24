package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"polyglot/internal/config"
	"polyglot/internal/data"
	"polyglot/internal/domain"
	"polyglot/pkg/logger"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Server:   config.ServerConfig{CORS: []string{"*"}},
		Backend:  config.BackendConfig{Provider: "uipath"},
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "data.db")},
		Log:      config.LogConfig{Level: "error", Format: "json"},
	}
	t.Chdir(filepath.Join("..", "..", ".."))
	srv, err := New(cfg, logger.New(cfg.Log))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func seedAPIKey(t *testing.T, srv *Server, rawKey, scopes string) {
	t.Helper()
	if _, err := srv.dataStore.Identity().UpsertAPIKey(context.Background(), domain.APIKey{
		ID:     "key_" + strings.ReplaceAll(rawKey, "-", "_"),
		UserID: "user_1",
		Name:   "test",
		Key:    rawKey,
		Scopes: scopes,
		Status: domain.StatusActive,
	}); err != nil {
		t.Fatalf("seed api key: %v", err)
	}
}

func setBearer(req *http.Request, rawKey string) {
	req.Header.Set("Authorization", "Bearer "+rawKey)
}

func bootstrapAdminToken(t *testing.T, srv *Server) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/bootstrap", strings.NewReader(`{
		"user_id":"user_1",
		"email":"admin@example.com",
		"display_name":"Admin",
		"password":"correct horse battery staple"
	}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode bootstrap response: %v", err)
	}
	if out.Token == "" {
		t.Fatalf("bootstrap did not return token: %s", w.Body.String())
	}
	return out.Token
}

func TestRouterSupportsGeminiModelsPath(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1beta/models/gemini-pro:generateContent", http.NoBody)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected models path to be routed")
	}
}

func TestAdminEndpointsExist(t *testing.T) {
	srv := newTestServer(t)
	adminToken := bootstrapAdminToken(t, srv)
	for _, path := range []string{
		"/api/admin/stats",
		"/api/admin/users",
		"/api/admin/roles",
		"/api/admin/user-roles?user_id=missing",
		"/api/admin/api-keys",
		"/api/admin/adapters",
		"/api/admin/adapter-instances",
		"/api/admin/providers",
		"/api/admin/model-mappings",
		"/api/admin/request-logs",
		"/api/admin/usage-events",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		setBearer(req, adminToken)
		srv.router.ServeHTTP(w, req)
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d", path, w.Code)
		}
	}
}

func TestMetricsEndpointExposesTelemetry(t *testing.T) {
	srv := newTestServer(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `polyglot_http_request_latency_ms_count{method="GET",path="/health",status="200"`) {
		t.Fatalf("metrics missing health request counter: %s", w.Body.String())
	}
}

func TestAdminIdentityCanBeManaged(t *testing.T) {
	srv := newTestServer(t)
	adminToken := bootstrapAdminToken(t, srv)
	posts := []struct {
		path string
		body string
	}{
		{"/api/admin/users", `{"id":"user_1","email":"a@example.com","display_name":"Alice"}`},
		{"/api/admin/roles", `{"id":"role_1","name":"admin","permissions":"[\"*\"]"}`},
		{"/api/admin/user-roles", `{"user_id":"user_1","role_id":"role_1"}`},
	}
	for _, post := range posts {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, post.path, strings.NewReader(post.body))
		req.Header.Set("Content-Type", "application/json")
		setBearer(req, adminToken)
		srv.router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", post.path, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "user_1") {
		t.Fatalf("users status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/api-keys", strings.NewReader(`{"id":"key_1","user_id":"user_1","name":"primary","scopes":"[\"openai\"]"}`))
	req.Header.Set("Content-Type", "application/json")
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("api key status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode api key response: %v", err)
	}
	if created.Key == "" {
		t.Fatalf("created api key did not return one-time key: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/api-keys", http.NoBody)
	setBearer(req, created.Key)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("api key used as admin session status=%d, want 401", w.Code)
	}
}

func TestAdminUpsertsGenerateStableIDs(t *testing.T) {
	srv := newTestServer(t)
	adminToken := bootstrapAdminToken(t, srv)

	providerID := postAndRequireID(t, srv, adminToken, "/api/admin/providers", `{
		"name":"Generated Provider",
		"type":"openai",
		"base_url":"https://api.openai.com",
		"auth_type":"bearer",
		"default_headers":"{}"
	}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/providers", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"id":"`+providerID+`"`) {
		t.Fatalf("generated provider id not listed: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/api-keys", strings.NewReader(`{"name":"generated","scopes":"[\"admin\"]"}`))
	req.Header.Set("Content-Type", "application/json")
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create generated api key status=%d body=%s", w.Code, w.Body.String())
	}
	var apiKey struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &apiKey); err != nil {
		t.Fatalf("decode generated api key response: %v", err)
	}
	if apiKey.ID == "" || apiKey.Key == "" {
		t.Fatalf("generated api key response missing id/key: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/api-keys", http.NoBody)
	setBearer(req, apiKey.Key)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("api key with admin scope used as admin session status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/api-keys", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"id":"`+apiKey.ID+`"`) {
		t.Fatalf("generated api key id not listed: status=%d body=%s", w.Code, w.Body.String())
	}
}

func postAndRequireID(t *testing.T, srv *Server, adminToken, path, body string) string {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode %s response: %v", path, err)
	}
	if created.ID == "" {
		t.Fatalf("%s returned empty id: %s", path, w.Body.String())
	}
	return created.ID
}

func TestAdminProvidersCanBeUpsertedAndListed(t *testing.T) {
	srv := newTestServer(t)
	adminToken := bootstrapAdminToken(t, srv)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/providers", strings.NewReader(`{
		"id":"prov_openai",
		"name":"OpenAI",
		"type":"openai",
		"base_url":"https://api.openai.com",
		"auth_type":"bearer",
		"default_headers":"{}"
	}`))
	req.Header.Set("Content-Type", "application/json")
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST providers status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/providers", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET providers status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "prov_openai") {
		t.Fatalf("provider not listed: %s", w.Body.String())
	}
}

func TestAdminStatsAndLogsUseDataStore(t *testing.T) {
	srv := newTestServer(t)
	adminToken := bootstrapAdminToken(t, srv)
	now := time.Now().UTC()
	if err := srv.dataStore.Audit().RecordRequest(context.Background(), domain.RequestLog{
		ID:         "req_1",
		Provider:   "openai",
		Protocol:   "openai",
		StatusCode: 200,
		Success:    true,
		LatencyMs:  50,
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("RecordRequest: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"requests_total":1`) {
		t.Fatalf("stats not data-backed: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/request-logs", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logs status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "req_1") {
		t.Fatalf("request log not listed: %s", w.Body.String())
	}
}

func TestProtocolRequestIsAudited(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	seedAPIKey(t, srv, "sk-anthropic", `["anthropic"]`)
	setBearer(req, "sk-anthropic")
	srv.router.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Fatalf("messages route not registered")
	}

	logs, err := srv.dataStore.Audit().ListRequestLogs(context.Background(), data.RequestLogFilter{
		Provider: "uipath",
		Protocol: "anthropic",
	})
	if err != nil {
		t.Fatalf("ListRequestLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("audited logs=%d, want 1", len(logs))
	}
}

func TestAPIKeyAuthRejectsMissingAndInsufficientScopes(t *testing.T) {
	srv := newTestServer(t)
	seedAPIKey(t, srv, "sk-admin", `["admin"]`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status=%d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4"}`))
	req.Header.Set("Content-Type", "application/json")
	setBearer(req, "sk-admin")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("wrong scope status=%d, want 403", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/stats", http.NoBody)
	setBearer(req, "sk-admin")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("api key used as admin session status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminAPIKeyCanBeViewedAfterCreation(t *testing.T) {
	srv := newTestServer(t)
	adminToken := bootstrapAdminToken(t, srv)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/api-keys", strings.NewReader(`{"id":"key_admin","name":"admin","scopes":"[\"admin\"]"}`))
	req.Header.Set("Content-Type", "application/json")
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create key status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create key: %v", err)
	}
	if created.Key == "" {
		t.Fatalf("new key missing from create response: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/api-keys", http.NoBody)
	setBearer(req, created.Key)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("api key used as admin session status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/api-keys", http.NoBody)
	setBearer(req, adminToken)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list keys status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), created.Key) {
		t.Fatalf("api key not visible in list response: %s", w.Body.String())
	}
}

func TestAdminLoginUsesPasswordSession(t *testing.T) {
	srv := newTestServer(t)
	bootstrapAdminToken(t, srv)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin@example.com","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin@example.com","password":"correct horse battery staple"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	var loggedIn struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loggedIn); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loggedIn.Token == "" {
		t.Fatalf("login did not return token: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/stats", http.NoBody)
	setBearer(req, loggedIn.Token)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("session access status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/profile", http.NoBody)
	setBearer(req, loggedIn.Token)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"email":"admin@example.com"`) {
		t.Fatalf("profile status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/logout", http.NoBody)
	setBearer(req, loggedIn.Token)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/stats", http.NoBody)
	setBearer(req, loggedIn.Token)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBootstrapAllowedForPasswordlessLegacyUsers(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.dataStore.Identity().UpsertUser(context.Background(), domain.User{
		ID:          "user_legacy",
		Email:       "legacy@example.com",
		DisplayName: "Legacy Admin",
		Status:      domain.StatusActive,
	}); err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/bootstrap", strings.NewReader(`{
		"user_id":"user_legacy",
		"email":"legacy@example.com",
		"password":"new password"
	}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("legacy bootstrap status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/bootstrap", strings.NewReader(`{
		"email":"second@example.com",
		"password":"another password"
	}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("second bootstrap status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRouterPassthroughOpenAIBypassesAdapter(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_direct"}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{CORS: []string{"*"}},
		Backend: config.BackendConfig{
			Provider: "passthrough",
			Passthrough: config.PassthroughConfig{
				Upstreams: map[string]config.UpstreamConfig{
					"openai": {URL: upstream.URL, APIKey: "sk-direct"},
				},
			},
		},
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "data.db")},
		Log:      config.LogConfig{Level: "error", Format: "json"},
	}
	t.Chdir(filepath.Join("..", "..", ".."))
	srv, err := New(cfg, logger.New(cfg.Log))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4"}`))
	req.Header.Set("Content-Type", "application/json")
	seedAPIKey(t, srv, "sk-openai", `["openai"]`)
	setBearer(req, "sk-openai")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q", gotPath)
	}
	if gotAuth != "Bearer sk-direct" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody != `{"model":"gpt-4"}` {
		t.Fatalf("body = %q", gotBody)
	}
	if !strings.Contains(w.Body.String(), "chatcmpl_direct") {
		t.Fatalf("response not proxied: %s", w.Body.String())
	}
}
