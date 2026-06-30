package account

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"polyglot/internal/data"
	"polyglot/internal/domain"
	pb "polyglot/proto/adapter"
)

// fakeProxyResolver 按 provider 名返回预置代理 URL，验证注册/心跳的配置下发。
type fakeProxyResolver struct {
	byProvider map[string]string
	calls      int
}

func (f *fakeProxyResolver) ResolveForProviderName(_ context.Context, name string) (string, error) {
	f.calls++
	return f.byProvider[name], nil
}

func openTestStore(t *testing.T) *data.Store {
	t.Helper()
	store, err := data.Open(data.Config{
		Driver:      data.DriverSQLite,
		DSN:         filepath.Join(t.TempDir(), "data.db"),
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("Open data store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// 注册响应应携带按 provider 解析出的代理 URL。
func TestResolveProxyURLOnRegisterResponse(t *testing.T) {
	svc := NewAccountPoolService()
	svc.SetProxyResolver(&fakeProxyResolver{byProvider: map[string]string{"uipath": "http://proxy.example:8080"}})

	if got := svc.resolveProxyURL(context.Background(), "uipath"); got != "http://proxy.example:8080" {
		t.Fatalf("resolveProxyURL=%q, want http://proxy.example:8080", got)
	}
	// 未配置代理的 provider → 空串（不报错）。
	if got := svc.resolveProxyURL(context.Background(), "unknown"); got != "" {
		t.Fatalf("resolveProxyURL(unknown)=%q, want empty", got)
	}
	// 无解析器时安全返回空串。
	bare := NewAccountPoolService()
	if got := bare.resolveProxyURL(context.Background(), "uipath"); got != "" {
		t.Fatalf("resolveProxyURL without resolver=%q, want empty", got)
	}
}

// 心跳应更新实例 LastHeartbeatAt 并回传最新代理配置。
func TestHeartbeatUpdatesInstanceAndReturnsConfig(t *testing.T) {
	store := openTestStore(t)
	svc := NewAccountPoolServiceWithStore(store)
	svc.SetProxyResolver(&fakeProxyResolver{byProvider: map[string]string{"uipath": "http://p:1"}})

	// 预置一个 source + 实例（LastHeartbeatAt 设为远古，模拟久未心跳）。
	svc.sources["uipath-1"] = &AccountSource{SourceID: "uipath-1", Provider: "uipath"}
	old := time.Now().UTC().Add(-time.Hour)
	if err := store.Adapters().UpsertInstance(context.Background(), domain.AdapterInstance{
		ID:              "uipath-1",
		AdapterID:       "uipath",
		Provider:        "uipath",
		Status:          domain.StatusStale,
		LastHeartbeatAt: &old,
	}); err != nil {
		t.Fatalf("UpsertInstance: %v", err)
	}

	resp, err := svc.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		SourceId:   "uipath-1",
		InstanceId: "uipath-1",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Heartbeat success=false")
	}
	if got := resp.GetConfig().GetProxyUrl(); got != "http://p:1" {
		t.Fatalf("heartbeat config proxy=%q, want http://p:1", got)
	}

	inst, found, err := store.Adapters().GetInstance(context.Background(), "uipath-1")
	if err != nil || !found {
		t.Fatalf("GetInstance found=%v err=%v", found, err)
	}
	if inst.Status != domain.StatusActive {
		t.Fatalf("instance status=%q, want active (MarkHeartbeat should revive)", inst.Status)
	}
	if inst.LastHeartbeatAt == nil || !inst.LastHeartbeatAt.After(old) {
		t.Fatalf("LastHeartbeatAt not advanced: %v", inst.LastHeartbeatAt)
	}
}

// reaper 应将心跳超时的实例标记为 stale，活跃实例保持不变。
func TestReapStaleInstances(t *testing.T) {
	store := openTestStore(t)
	svc := NewAccountPoolServiceWithStore(store)

	now := time.Now().UTC()
	stale := now.Add(-2 * time.Minute) // 超过 60s 阈值
	fresh := now.Add(-5 * time.Second) // 阈值内

	if err := store.Adapters().UpsertInstance(context.Background(), domain.AdapterInstance{
		ID: "stale-1", AdapterID: "uipath", Provider: "uipath",
		Status: domain.StatusActive, LastHeartbeatAt: &stale,
	}); err != nil {
		t.Fatalf("UpsertInstance stale: %v", err)
	}
	if err := store.Adapters().UpsertInstance(context.Background(), domain.AdapterInstance{
		ID: "fresh-1", AdapterID: "uipath", Provider: "uipath",
		Status: domain.StatusActive, LastHeartbeatAt: &fresh,
	}); err != nil {
		t.Fatalf("UpsertInstance fresh: %v", err)
	}

	svc.reapStaleInstances()

	staleInst, _, err := store.Adapters().GetInstance(context.Background(), "stale-1")
	if err != nil {
		t.Fatalf("GetInstance stale: %v", err)
	}
	if staleInst.Status != domain.StatusStale {
		t.Fatalf("stale instance status=%q, want stale", staleInst.Status)
	}

	freshInst, _, err := store.Adapters().GetInstance(context.Background(), "fresh-1")
	if err != nil {
		t.Fatalf("GetInstance fresh: %v", err)
	}
	if freshInst.Status != domain.StatusActive {
		t.Fatalf("fresh instance status=%q, want active (within threshold)", freshInst.Status)
	}
}
