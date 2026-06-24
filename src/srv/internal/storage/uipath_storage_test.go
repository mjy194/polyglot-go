package storage

import (
	"context"
	"testing"

	"polyglot/internal/data"
	pb "polyglot/proto/adapter"
)

func newTestKVService(t *testing.T) *UiPathStorageService {
	t.Helper()
	store, err := data.Open(data.Config{Driver: data.DriverSQLite, DSN: ":memory:", AutoMigrate: true})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewUiPathStorageServiceWithStore(store)
}

func TestKVPutGetRoundTrip(t *testing.T) {
	svc := newTestKVService(t)
	ctx := context.Background()

	if _, err := svc.Put(ctx, &pb.PutRequest{
		SourceId: "uipath", Key: "auth/a@example.com",
		Value: []byte(`{"access_token":"token-1"}`), ExpiresAt: 100,
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := svc.Get(ctx, &pb.GetRequest{SourceId: "uipath", Key: "auth/a@example.com"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Found || string(got.Value) != `{"access_token":"token-1"}` || got.ExpiresAt != 100 {
		t.Fatalf("unexpected Get result: %+v", got)
	}
}

func TestKVPutOverwrites(t *testing.T) {
	svc := newTestKVService(t)
	ctx := context.Background()

	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "uipath", Key: "k", Value: []byte("v1")})
	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "uipath", Key: "k", Value: []byte("v2")})

	got, _ := svc.Get(ctx, &pb.GetRequest{SourceId: "uipath", Key: "k"})
	if !got.Found || string(got.Value) != "v2" {
		t.Fatalf("expected overwrite to v2, got %+v", got)
	}
}

func TestKVDelete(t *testing.T) {
	svc := newTestKVService(t)
	ctx := context.Background()

	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "uipath", Key: "k", Value: []byte("v")})
	if _, err := svc.Delete(ctx, &pb.DeleteRequest{SourceId: "uipath", Key: "k"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ := svc.Get(ctx, &pb.GetRequest{SourceId: "uipath", Key: "k"})
	if got.Found {
		t.Fatalf("expected record deleted, still found")
	}
}

func TestKVListPrefix(t *testing.T) {
	svc := newTestKVService(t)
	ctx := context.Background()

	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "uipath", Key: "auth/a", Value: []byte("1")})
	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "uipath", Key: "auth/b", Value: []byte("2")})
	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "uipath", Key: "other/c", Value: []byte("3")})
	_, _ = svc.Put(ctx, &pb.PutRequest{SourceId: "anthropic", Key: "auth/a", Value: []byte("x")})

	resp, err := svc.List(ctx, &pb.ListRequest{SourceId: "uipath", Prefix: "auth/"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 auth/ entries under uipath, got %d", len(resp.Entries))
	}
}
