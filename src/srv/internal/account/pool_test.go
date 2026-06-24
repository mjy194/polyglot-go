package account

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"polyglot/internal/data"
	"polyglot/internal/domain"
	pb "polyglot/proto/adapter"

	"google.golang.org/grpc"
)

// fakeSourceClient 实现 pb.AccountSourceServiceClient，记录 SupplyAccounts 调用次数；
// 通过 block channel 可让供应阻塞，以便测试"在途期间不重复触发"。
type fakeSourceClient struct {
	mu     sync.Mutex
	calls  int
	block  chan struct{}
	accTTL time.Duration
}

func (f *fakeSourceClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSourceClient) SupplyAccounts(ctx context.Context, req *pb.SupplyAccountsRequest, opts ...grpc.CallOption) (*pb.SupplyAccountsResponse, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
		}
	}

	ttl := f.accTTL
	if ttl == 0 {
		ttl = time.Hour
	}
	var accs []*pb.AccountInfo
	for i := 0; i < int(req.Count); i++ {
		accs = append(accs, &pb.AccountInfo{
			AccountId: fmt.Sprintf("acc-%d-%d", f.callCount(), i),
			ExpiresAt: time.Now().Add(ttl).Unix(),
		})
	}
	return &pb.SupplyAccountsResponse{SuppliedCount: int32(len(accs)), Accounts: accs, Message: "fake"}, nil
}

func (f *fakeSourceClient) ListAccounts(context.Context, *pb.ListAccountsRequest, ...grpc.CallOption) (*pb.ListAccountsResponse, error) {
	return &pb.ListAccountsResponse{}, nil
}
func (f *fakeSourceClient) RefreshAccount(context.Context, *pb.RefreshAccountRequest, ...grpc.CallOption) (*pb.RefreshAccountResponse, error) {
	return &pb.RefreshAccountResponse{}, nil
}
func (f *fakeSourceClient) HealthCheck(context.Context, *pb.AccountHealthCheckRequest, ...grpc.CallOption) (*pb.AccountHealthCheckResponse, error) {
	return &pb.AccountHealthCheckResponse{}, nil
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

func watermark(low, high, batch, max int32) *pb.WatermarkConfig {
	return &pb.WatermarkConfig{MinAccounts: 1, MaxAccounts: max, LowWatermark: low, HighWatermark: high, SupplyBatchSize: batch}
}

// 补号在途期间，反复触发 checkWatermark 不应再发起 SupplyAccounts。
func TestCheckWatermarkNoConcurrentSupply(t *testing.T) {
	svc := NewAccountPoolService()
	block := make(chan struct{})
	fc := &fakeSourceClient{block: block}
	svc.sources["s"] = &AccountSource{
		SourceID: "s", Provider: "uipath", Client: fc,
		Watermark: watermark(3, 8, 2, 10),
	}

	// 第一次：0<3，发起补号（goroutine 阻塞在 block）
	svc.checkWatermark()
	waitFor(t, func() bool { return fc.callCount() == 1 }, time.Second, "first supply to start")

	// 在途期间反复触发 —— 全部应跳过
	for i := 0; i < 5; i++ {
		svc.checkWatermark()
	}
	if got := fc.callCount(); got != 1 {
		t.Fatalf("补号在途期间应只有 1 次供应，实际 %d 次（节流失效）", got)
	}

	// 放行补号，等它结束并清除标记
	close(block)
	waitFor(t, func() bool {
		svc.mu.RLock()
		defer svc.mu.RUnlock()
		return !svc.sources["s"].supplying
	}, time.Second, "supply to finish and clear flag")
}

// 补号量按 MaxAccounts 封顶；达到/超过低水位后不再补号（不超额）。
func TestCheckWatermarkConvergesWithoutOvershoot(t *testing.T) {
	svc := NewAccountPoolService()
	fc := &fakeSourceClient{} // 不阻塞，按 req.Count 返回健康账号
	svc.sources["s"] = &AccountSource{
		SourceID: "s", Provider: "uipath", Client: fc,
		Watermark: watermark(3, 8, 2, 10),
	}

	// 串行触发若干次，每次给补号 goroutine 足够时间完成
	for i := 0; i < 6; i++ {
		svc.checkWatermark()
		time.Sleep(20 * time.Millisecond)
	}

	svc.mu.RLock()
	healthy := svc.countHealthyAccountsLocked("s")
	svc.mu.RUnlock()

	// LowWatermark=3，batch=2：应稳定在 [3, LowWatermark+batch]=[3,5]，绝不会冲到 HighWatermark(8) 以上
	if healthy < 3 || healthy > 5 {
		t.Errorf("healthy=%d，期望收敛到 [3,5]（不超额）", healthy)
	}
	if got := fc.callCount(); got > 2 {
		t.Errorf("供应次数=%d，期望 ≤2 即可达低水位（节流后不再重复触发）", got)
	}
}

// healthy 已达 MaxAccounts 时即使低于 LowWatermark 也不补号（硬上限）。
func TestCheckWatermarkRespectsMaxAccounts(t *testing.T) {
	svc := NewAccountPoolService()
	fc := &fakeSourceClient{}
	src := &AccountSource{
		SourceID: "s", Provider: "uipath", Client: fc,
		Watermark: &pb.WatermarkConfig{MinAccounts: 1, MaxAccounts: 2, LowWatermark: 5, HighWatermark: 8, SupplyBatchSize: 2},
	}
	svc.sources["s"] = src
	// 预置 2 个健康账号 = MaxAccounts
	svc.mu.Lock()
	svc.accounts["a1"] = &Account{AccountID: "a1", SourceID: "s", Provider: "uipath", Health: "healthy", ExpiresAt: time.Now().Add(time.Hour)}
	svc.accounts["a2"] = &Account{AccountID: "a2", SourceID: "s", Provider: "uipath", Health: "healthy", ExpiresAt: time.Now().Add(time.Hour)}
	svc.mu.Unlock()

	svc.checkWatermark()
	time.Sleep(20 * time.Millisecond)

	if got := fc.callCount(); got != 0 {
		t.Errorf("已达 MaxAccounts 仍发起了 %d 次补号（应被硬上限拦截）", got)
	}
}

func TestAccountPoolPersistsAccountLifecycle(t *testing.T) {
	store, err := data.Open(data.Config{
		Driver:      data.DriverSQLite,
		DSN:         filepath.Join(t.TempDir(), "data.db"),
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("Open data store: %v", err)
	}
	defer store.Close()

	svc := NewAccountPoolServiceWithStore(store)
	source := &AccountSource{
		SourceID:     "src_1",
		Provider:     "uipath",
		CallbackAddr: "127.0.0.1:50051",
		Capabilities: []string{"chat"},
		Watermark:    watermark(1, 2, 1, 2),
		LastCheckAt:  time.Now(),
	}
	svc.sources[source.SourceID] = source
	svc.persistSourceLocked(context.Background(), source)
	svc.addAccountLocked(source.SourceID, &pb.AccountInfo{
		AccountId: "acc_1",
		Credentials: map[string]string{
			"access_token": "token",
		},
		Metadata: map[string]string{
			"email": "a@example.com",
		},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	svc.persistAccountsLocked(context.Background(), source.SourceID, []*pb.AccountInfo{{
		AccountId:   "acc_1",
		Credentials: map[string]string{"access_token": "token"},
		Metadata:    map[string]string{"email": "a@example.com"},
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}})

	if _, found, err := store.Accounts().GetSource(context.Background(), "src_1"); err != nil || !found {
		t.Fatalf("persisted source found=%v err=%v", found, err)
	}
	if instances, err := store.Adapters().ListInstances(context.Background(), "uipath"); err != nil || len(instances) != 1 {
		t.Fatalf("persisted adapter instances len=%d err=%v", len(instances), err)
	}

	acquired, err := svc.AcquireAccount(context.Background(), &pb.AcquireAccountRequest{
		Provider:  "uipath",
		SessionId: "session_1",
	})
	if err != nil || !acquired.Available {
		t.Fatalf("AcquireAccount available=%v err=%v", acquired.GetAvailable(), err)
	}
	var lease data.AccountLeaseRecord
	if err := store.DB().First(&lease, "account_id = ? AND session_id = ?", "acc_1", "session_1").Error; err != nil {
		t.Fatalf("load persisted lease: %v", err)
	}

	if _, err := svc.ReleaseAccount(context.Background(), &pb.ReleaseAccountRequest{AccountId: "acc_1"}); err != nil {
		t.Fatalf("ReleaseAccount: %v", err)
	}
	if err := store.DB().First(&lease, "id = ?", lease.ID).Error; err != nil {
		t.Fatalf("reload lease: %v", err)
	}
	if lease.Status != domain.LeaseStatusReleased {
		t.Fatalf("lease status=%q, want released", lease.Status)
	}

	if _, err := svc.ReportUsage(context.Background(), &pb.ReportUsageRequest{
		AccountId:     "acc_1",
		TokensUsed:    12,
		RequestsCount: 1,
	}); err != nil {
		t.Fatalf("ReportUsage: %v", err)
	}
	events, err := store.Audit().ListUsageEvents(context.Background(), data.UsageEventFilter{AccountID: "acc_1"})
	if err != nil {
		t.Fatalf("ListUsageEvents: %v", err)
	}
	if len(events) != 1 || events[0].TokensUsed != 12 {
		t.Fatalf("unexpected usage events: %+v", events)
	}
}
