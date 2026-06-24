package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	pb "polyglot/proto/adapter"
)

func TestSaveAuthStateMergesExistingRow(t *testing.T) {
	svc, err := NewUiPathStorageService(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("NewUiPathStorageService: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()
	first := &pb.SaveAuthStateRequest{
		Email:        "a@example.com",
		AccessToken:  "token-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		UpstreamUrl:  "https://example.test/1",
	}
	if _, err := svc.SaveAuthState(ctx, first); err != nil {
		t.Fatalf("first SaveAuthState: %v", err)
	}

	second := &pb.SaveAuthStateRequest{
		Email:        "a@example.com",
		AccessToken:  "token-2",
		RefreshToken: "refresh-2",
		ExpiresAt:    time.Now().Add(2 * time.Hour).Unix(),
		UpstreamUrl:  "https://example.test/2",
	}
	if _, err := svc.SaveAuthState(ctx, second); err != nil {
		t.Fatalf("second SaveAuthState: %v", err)
	}

	got, err := svc.LoadAuthState(ctx, &pb.LoadAuthStateRequest{
		Email: "a@example.com",
	})
	if err != nil {
		t.Fatalf("LoadAuthState: %v", err)
	}
	if !got.Found {
		t.Fatalf("expected state to be found")
	}
	if got.AccessToken != "token-2" || got.RefreshToken != "refresh-2" || got.UpstreamUrl != "https://example.test/2" {
		t.Fatalf("loaded wrong state: %+v", got)
	}
}
