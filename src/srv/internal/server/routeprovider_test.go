package server

import (
	"context"
	"testing"

	"polyglot/internal/domain"
	"polyglot/internal/passthrough"
)

// upsertProv is a tiny helper to seed a provider with the given fields.
func upsertProv(t *testing.T, srv *Server, name, typ, mode, adapter string) *domain.Provider {
	t.Helper()
	p, err := srv.dataStore.Providers().UpsertProvider(context.Background(), domain.Provider{
		Name: name, Type: typ, BaseURL: "https://x", Status: domain.StatusActive,
		Mode: mode, Adapter: adapter,
	})
	if err != nil {
		t.Fatalf("UpsertProvider %s: %v", name, err)
	}
	return &p
}

func TestRouteProviderEmptyModeIsPassthrough(t *testing.T) {
	srv := newTestServer(t)
	// Empty Mode now defaults to passthrough (not "legacy skip"), so a provider
	// with matching Type resolves.
	upsertProv(t, srv, "default", "anthropic", "", "")
	p, ok := srv.routeProvider(context.Background(), passthrough.ProtocolAnthropic, "default")
	if !ok || p == nil || p.Name != "default" {
		t.Fatalf("expected empty-mode provider to resolve as passthrough, got %+v ok=%v", p, ok)
	}
	// Different protocol with no matching provider → nil (legacy config fallback).
	if p, ok := srv.routeProvider(context.Background(), passthrough.ProtocolGemini, "default"); ok || p != nil {
		t.Fatalf("gemini should not resolve, got %+v", p)
	}
}

func TestRouteProviderPassthroughMatchesByType(t *testing.T) {
	srv := newTestServer(t)
	upsertProv(t, srv, "direct-openai", "openai", "passthrough", "")
	// responses falls back to openai type
	for _, proto := range []string{passthrough.ProtocolOpenAI, passthrough.ProtocolResponses} {
		p, ok := srv.routeProvider(context.Background(), proto, "default")
		if !ok || p == nil || p.Mode != "passthrough" {
			t.Fatalf("proto %s: expected passthrough provider, got %+v ok=%v", proto, p, ok)
		}
	}
	// anthropic not configured → no passthrough match
	if p, ok := srv.routeProvider(context.Background(), passthrough.ProtocolAnthropic, "default"); ok || p != nil {
		t.Fatalf("anthropic should not resolve, got %+v", p)
	}
}

func TestRouteProviderAdapterFallback(t *testing.T) {
	srv := newTestServer(t)
	// Only an adapter-mode provider → any protocol resolves to it.
	upsertProv(t, srv, "uipath", "uipath", "adapter", "uipath")
	p, ok := srv.routeProvider(context.Background(), passthrough.ProtocolAnthropic, "default")
	if !ok || p == nil || p.Mode != "adapter" || p.Adapter != "uipath" {
		t.Fatalf("expected adapter provider uipath, got %+v ok=%v", p, ok)
	}
}

func TestRouteProviderPassthroughOverridesAdapter(t *testing.T) {
	srv := newTestServer(t)
	upsertProv(t, srv, "uipath", "uipath", "adapter", "uipath")
	upsertProv(t, srv, "direct-anthropic", "anthropic", "passthrough", "")
	// anthropic → passthrough wins over the generic adapter
	p, ok := srv.routeProvider(context.Background(), passthrough.ProtocolAnthropic, "default")
	if !ok || p.Mode != "passthrough" {
		t.Fatalf("anthropic should resolve to passthrough, got %+v ok=%v", p, ok)
	}
	// openai → no passthrough, falls to adapter
	p, ok = srv.routeProvider(context.Background(), passthrough.ProtocolOpenAI, "default")
	if !ok || p.Mode != "adapter" {
		t.Fatalf("openai should resolve to adapter, got %+v ok=%v", p, ok)
	}
}

func TestRouteProviderGroupScoped(t *testing.T) {
	srv := newTestServer(t)
	a := upsertProv(t, srv, "a", "anthropic", "passthrough", "")
	upsertProv(t, srv, "b", "anthropic", "passthrough", "")

	def, ok, _ := srv.dataStore.Groups().GetGroupByName(context.Background(), "default")
	if !ok {
		t.Fatal("default group missing")
	}
	if err := srv.dataStore.Groups().SetGroupProviders(context.Background(), def.ID,
		[]domain.GroupProvider{{ProviderID: a.ID, Priority: 0}}); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		p, ok := srv.routeProvider(context.Background(), passthrough.ProtocolAnthropic, "default")
		if !ok || p == nil || p.Name != "a" {
			t.Fatalf("expected group-scoped pick 'a', got %+v ok=%v", p, ok)
		}
	}
}
