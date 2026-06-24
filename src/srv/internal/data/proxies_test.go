package data

import (
	"context"
	"path/filepath"
	"testing"

	"polyglot/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestProxyRepositoryUpsertGetList(t *testing.T) {
	store := newTestStore(t)
	repo := store.Proxies()
	ctx := context.Background()

	p1, err := repo.UpsertProxy(ctx, domain.Proxy{Name: "primary", URL: "http://127.0.0.1:8888", Type: "http"})
	if err != nil {
		t.Fatalf("UpsertProxy: %v", err)
	}
	if p1.ID == "" || p1.Status != domain.StatusActive {
		t.Fatalf("unexpected proxy: %+v", p1)
	}

	// upsert by id updates
	p1.Status = domain.StatusDisabled
	if _, err := repo.UpsertProxy(ctx, p1); err != nil {
		t.Fatalf("UpsertProxy update: %v", err)
	}
	got, found, err := repo.GetProxy(ctx, p1.ID)
	if err != nil || !found || got.Status != domain.StatusDisabled {
		t.Fatalf("GetProxy after update: %+v found=%v err=%v", got, found, err)
	}

	if _, err := repo.UpsertProxy(ctx, domain.Proxy{Name: "socks", URL: "socks5://127.0.0.1:1080", Type: "socks5"}); err != nil {
		t.Fatalf("UpsertProxy second: %v", err)
	}
	list, err := repo.ListProxies(ctx)
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(list))
	}

	if err := repo.DeleteProxy(ctx, p1.ID); err != nil {
		t.Fatalf("DeleteProxy: %v", err)
	}
	if _, found, _ := repo.GetProxy(ctx, p1.ID); found {
		t.Fatalf("proxy should be deleted")
	}
}

func TestProviderProxiesReplaceAndOrder(t *testing.T) {
	store := newTestStore(t)
	repo := store.Proxies()
	ctx := context.Background()

	prov, err := store.Providers().UpsertProvider(ctx, domain.Provider{Name: "p", Type: "anthropic", BaseURL: "https://x", ProxyStrategy: "round_robin"})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	pa, _ := repo.UpsertProxy(ctx, domain.Proxy{Name: "a", URL: "http://a:1", Type: "http"})
	pb, _ := repo.UpsertProxy(ctx, domain.Proxy{Name: "b", URL: "http://b:1", Type: "http"})

	// initial set, with priorities out of insertion order
	if err := repo.SetProviderProxies(ctx, prov.ID, []domain.ProviderProxy{
		{ProxyID: pb.ID, Priority: 1},
		{ProxyID: pa.ID, Priority: 0},
	}); err != nil {
		t.Fatalf("SetProviderProxies: %v", err)
	}
	got, err := repo.ListProviderProxies(ctx, prov.ID)
	if err != nil {
		t.Fatalf("ListProviderProxies: %v", err)
	}
	if len(got) != 2 || got[0].ProxyID != pa.ID || got[1].ProxyID != pb.ID {
		t.Fatalf("expected priority-ordered [a,b], got %+v", got)
	}

	// replace: drop b, keep a — full replacement semantics
	if err := repo.SetProviderProxies(ctx, prov.ID, []domain.ProviderProxy{
		{ProxyID: pa.ID, Priority: 5},
	}); err != nil {
		t.Fatalf("SetProviderProxies replace: %v", err)
	}
	got, _ = repo.ListProviderProxies(ctx, prov.ID)
	if len(got) != 1 || got[0].ProxyID != pa.ID || got[0].Priority != 5 {
		t.Fatalf("expected replaced single [a prio5], got %+v", got)
	}

	// clear all
	if err := repo.SetProviderProxies(ctx, prov.ID, nil); err != nil {
		t.Fatalf("SetProviderProxies clear: %v", err)
	}
	got, _ = repo.ListProviderProxies(ctx, prov.ID)
	if len(got) != 0 {
		t.Fatalf("expected cleared, got %+v", got)
	}
}
