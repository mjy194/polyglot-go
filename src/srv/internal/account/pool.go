package account

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"polyglot/internal/data"
	"polyglot/internal/domain"
	pb "polyglot/proto/adapter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProxyResolver 按 provider 名解析出站代理 URL（provider↔proxy 的 M:N 选择）。
// 由 internal/server 的 storeProxyResolver 实现并注入，避免 account→server 的循环依赖。
type ProxyResolver interface {
	ResolveForProviderName(ctx context.Context, name string) (string, error)
}

// AccountPoolService 实现 AccountService 接口
type AccountPoolService struct {
	pb.UnimplementedAccountServiceServer
	sources  map[string]*AccountSource
	accounts map[string]*Account
	store    *data.Store
	mu       sync.RWMutex

	// 代理解析（注册响应 + 心跳回传时按 provider 选代理下发给 adapter）
	proxyResolver ProxyResolver

	// 水位监控
	ticker *time.Ticker
	stopCh chan struct{}

	// 实例存活回收（心跳超时标记 stale）
	reaperTicker *time.Ticker
}

// SetProxyResolver 注入代理解析器（在 server 装配阶段调用）。
func (s *AccountPoolService) SetProxyResolver(r ProxyResolver) {
	s.proxyResolver = r
}

// resolveProxyURL 按 provider 名解析单个代理 URL；无解析器或出错时返回空串。
func (s *AccountPoolService) resolveProxyURL(ctx context.Context, provider string) string {
	if s.proxyResolver == nil || provider == "" {
		return ""
	}
	url, err := s.proxyResolver.ResolveForProviderName(ctx, provider)
	if err != nil {
		log.Printf("⚠️  Failed to resolve proxy for provider %s: %v", provider, err)
		return ""
	}
	return url
}

// AccountSource 账号源
type AccountSource struct {
	SourceID      string
	Provider      string
	CallbackAddr  string
	Client        pb.AccountSourceServiceClient
	AdapterClient pb.AdapterServiceClient // 复用同一 conn，供主框架回调 adapter 的 ProcessRequest
	Metadata      *pb.AdapterMetadata
	Watermark     *pb.WatermarkConfig
	Capabilities  []string

	// supplying 标记当前是否已有补号请求在途。同一 source 同时只允许一个 SupplyAccounts，
	// 否则在慢速 OAuth 期间每个 ticker 周期都会重复触发，goroutine 叠加导致严重超额补号。
	supplying bool

	// 统计
	TotalAccounts   int
	HealthyAccounts int
	LastCheckAt     time.Time
}

// Account 账号实例
type Account struct {
	AccountID   string
	SourceID    string
	Provider    string
	Credentials map[string]string
	Metadata    map[string]string
	ExpiresAt   time.Time
	Quota       *pb.AccountQuota

	// 状态
	InUse      bool
	UsageCount int64
	LastUsedAt time.Time
	Health     string // "healthy", "rate_limited", "quota_exceeded", "auth_failed"
}

// NewAccountPoolService 创建账号池服务
func NewAccountPoolService() *AccountPoolService {
	return &AccountPoolService{
		sources:  make(map[string]*AccountSource),
		accounts: make(map[string]*Account),
		stopCh:   make(chan struct{}),
	}
}

func NewAccountPoolServiceWithStore(store *data.Store) *AccountPoolService {
	svc := NewAccountPoolService()
	svc.store = store
	return svc
}

// Start 启动水位监控 + 实例存活回收
func (s *AccountPoolService) Start() {
	log.Println("🚀 Starting account pool watermark monitor...")
	s.ticker = time.NewTicker(5 * time.Second)
	// reaper 周期远长于心跳间隔（adapter 每 ~10s 心跳一次），避免误标 stale。
	s.reaperTicker = time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.checkWatermark()
			case <-s.reaperTicker.C:
				s.reapStaleInstances()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop 停止服务
func (s *AccountPoolService) Stop() {
	close(s.stopCh)
	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.reaperTicker != nil {
		s.reaperTicker.Stop()
	}
}

// RegisterAccountSource 注册账号源
func (s *AccountPoolService) RegisterAccountSource(ctx context.Context, req *pb.RegisterSourceRequest) (*pb.RegisterSourceResponse, error) {
	log.Printf("📝 Registering account source: %s (provider=%s, addr=%s)", req.SourceId, req.Provider, req.CallbackAddr)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 连接到 adapter
	conn, err := grpc.NewClient(req.CallbackAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.RegisterSourceResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to connect to adapter: %v", err),
		}, nil
	}

	client := pb.NewAccountSourceServiceClient(conn)
	// 复用同一连接创建 AdapterService 客户端：adapter 的 AdapterService 与
	// AccountSourceService 共用同一个监听端口（CallbackAddr）。
	adapterClient := pb.NewAdapterServiceClient(conn)
	metadata := fetchAdapterMetadata(adapterClient)

	source := &AccountSource{
		SourceID:      req.SourceId,
		Provider:      req.Provider,
		CallbackAddr:  req.CallbackAddr,
		Client:        client,
		AdapterClient: adapterClient,
		Metadata:      metadata,
		Watermark:     req.Watermark,
		Capabilities:  req.Capabilities,
		LastCheckAt:   time.Now(),
	}

	s.sources[req.SourceId] = source
	s.persistSourceLocked(ctx, source)

	// 初始化：拉取账号列表
	listCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.ListAccounts(listCtx, &pb.ListAccountsRequest{
		SourceId: req.SourceId,
	})

	if err != nil {
		log.Printf("⚠️  Failed to list accounts from %s: %v", req.SourceId, err)
		return &pb.RegisterSourceResponse{
			Success: true,
			Message: fmt.Sprintf("Source registered but failed to fetch initial accounts: %v", err),
			Config:  &pb.SourceConfig{ProxyUrl: s.resolveProxyURL(ctx, req.Provider)},
		}, nil
	}

	for _, acc := range resp.Accounts {
		s.addAccountLocked(req.SourceId, acc)
	}
	s.persistAccountsLocked(ctx, req.SourceId, resp.Accounts)

	source.TotalAccounts = len(resp.Accounts)
	source.HealthyAccounts = s.countHealthyAccountsLocked(req.SourceId)

	log.Printf("✅ Registered source %s with %d accounts (watermark: %d/%d)",
		req.SourceId, len(resp.Accounts),
		req.Watermark.LowWatermark, req.Watermark.HighWatermark)

	return &pb.RegisterSourceResponse{
		Success: true,
		Message: fmt.Sprintf("Registered with %d accounts", len(resp.Accounts)),
		Config:  &pb.SourceConfig{ProxyUrl: s.resolveProxyURL(ctx, req.Provider)},
	}, nil
}

func fetchAdapterMetadata(client pb.AdapterServiceClient) *pb.AdapterMetadata {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metadata, err := client.GetMetadata(ctx, &pb.GetMetadataRequest{})
	if err != nil {
		log.Printf("⚠️  Failed to fetch adapter metadata: %v", err)
		return nil
	}
	return metadata
}

// AdapterClient 返回指定 provider 对应 adapter 的 AdapterService 客户端，
// 供 HTTP handler 回调 adapter 的 ProcessRequest。
//
// 动态寻址：adapter 在 RegisterAccountSource 时通过 CallbackAddr 上报地址，
// 主框架复用该连接建立客户端；若尚无该 provider 的 adapter 注册，ok=false。
func (s *AccountPoolService) AdapterClient(provider string) (pb.AdapterServiceClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, src := range s.sources {
		if src.Provider == provider && src.AdapterClient != nil {
			return src.AdapterClient, true
		}
	}
	return nil, false
}

// NativeAdapterClient returns an adapter client only when metadata declares native support.
func (s *AccountPoolService) NativeAdapterClient(provider, protocol, endpoint string) (pb.AdapterServiceClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, src := range s.sources {
		if src.Provider == provider && src.AdapterClient != nil && adapterSupportsNative(src.Metadata, protocol, endpoint) {
			return src.AdapterClient, true
		}
	}
	return nil, false
}

func adapterSupportsNative(metadata *pb.AdapterMetadata, protocol, endpoint string) bool {
	if metadata == nil {
		return false
	}
	for _, support := range metadata.GetNativeProtocols() {
		if support.GetProtocol() != protocol {
			continue
		}
		for _, candidate := range support.GetEndpoints() {
			if candidate == "*" || candidate == endpoint {
				return true
			}
		}
	}
	return false
}

// AcquireAccount 获取可用账号
func (s *AccountPoolService) AcquireAccount(ctx context.Context, req *pb.AcquireAccountRequest) (*pb.AcquireAccountResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 按 provider 过滤
	candidates := s.filterByProviderLocked(req.Provider)

	// 应用自定义过滤条件
	candidates = s.applyFiltersLocked(candidates, req.Filters)

	// 会话亲和性：优先返回同一 session 上次使用的账号
	if req.SessionId != "" {
		for _, acc := range candidates {
			if acc.Metadata["last_session_id"] == req.SessionId && acc.Health == "healthy" && !acc.InUse {
				acc.InUse = true
				acc.LastUsedAt = time.Now()
				acc.UsageCount++
				acc.Metadata["last_session_id"] = req.SessionId
				s.persistAcquiredLocked(ctx, acc, req.SessionId)

				log.Printf("🔄 Reusing account %s for session %s", acc.AccountID, req.SessionId)

				return &pb.AcquireAccountResponse{
					Available:   true,
					AccountId:   acc.AccountID,
					Credentials: acc.Credentials,
					Metadata:    acc.Metadata,
				}, nil
			}
		}
	}

	// 选择最优账号：健康、未使用、使用次数最少
	best := s.selectBestAccountLocked(candidates)
	if best == nil {
		log.Printf("⚠️  No available account for provider=%s", req.Provider)
		return &pb.AcquireAccountResponse{Available: false}, nil
	}

	best.InUse = true
	best.LastUsedAt = time.Now()
	best.UsageCount++
	if req.SessionId != "" {
		best.Metadata["last_session_id"] = req.SessionId
	}
	s.persistAcquiredLocked(ctx, best, req.SessionId)

	log.Printf("✅ Acquired account %s (provider=%s, usage=%d)", best.AccountID, req.Provider, best.UsageCount)

	return &pb.AcquireAccountResponse{
		Available:   true,
		AccountId:   best.AccountID,
		Credentials: best.Credentials,
		Metadata:    best.Metadata,
	}, nil
}

// ReleaseAccount 释放账号
func (s *AccountPoolService) ReleaseAccount(ctx context.Context, req *pb.ReleaseAccountRequest) (*pb.ReleaseAccountResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[req.AccountId]
	if !ok {
		return &pb.ReleaseAccountResponse{Success: false}, nil
	}

	acc.InUse = false
	if s.store != nil {
		if err := s.store.Accounts().ReleaseAccount(ctx, req.AccountId); err != nil {
			log.Printf("⚠️  Failed to persist account release %s: %v", req.AccountId, err)
		}
	}
	log.Printf("🔓 Released account %s", req.AccountId)

	return &pb.ReleaseAccountResponse{Success: true}, nil
}

// ReportUsage 上报使用情况
func (s *AccountPoolService) ReportUsage(ctx context.Context, req *pb.ReportUsageRequest) (*pb.ReportUsageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[req.AccountId]
	if !ok {
		return &pb.ReportUsageResponse{Success: false}, nil
	}

	if acc.Quota != nil {
		acc.Quota.Used += req.TokensUsed
	}
	if s.store != nil {
		if err := s.store.Accounts().RecordUsage(ctx, domain.UsageEvent{
			ID:            fmt.Sprintf("usage_%s_%d", req.AccountId, time.Now().UnixNano()),
			AccountID:     req.AccountId,
			Provider:      acc.Provider,
			TokensUsed:    req.TokensUsed,
			RequestsCount: req.RequestsCount,
			CreatedAt:     time.Now().UTC(),
		}); err != nil {
			log.Printf("⚠️  Failed to persist account usage %s: %v", req.AccountId, err)
		}
	}

	return &pb.ReportUsageResponse{Success: true}, nil
}

// Heartbeat 处理 adapter 的周期心跳：更新实例存活时间，并回传最新配置（代理）实现热更新。
func (s *AccountPoolService) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	instanceID := req.InstanceId
	if instanceID == "" {
		instanceID = req.SourceId
	}
	if s.store != nil && instanceID != "" {
		if err := s.store.Adapters().MarkHeartbeat(ctx, instanceID, time.Now().UTC()); err != nil {
			log.Printf("⚠️  Failed to mark heartbeat for instance %s: %v", instanceID, err)
		}
	}

	// 回传最新代理配置（按 source 的 provider 解析），adapter 据此热更新登录/续期出口。
	s.mu.RLock()
	provider := ""
	if src := s.sources[req.SourceId]; src != nil {
		provider = src.Provider
	}
	s.mu.RUnlock()

	return &pb.HeartbeatResponse{
		Success: true,
		Config:  &pb.SourceConfig{ProxyUrl: s.resolveProxyURL(ctx, provider)},
	}, nil
}

// reapStaleInstances 将心跳超时的实例标记为 stale（激活 SetInstanceStatus）。
// 心跳间隔约 10s，阈值取 60s 留足余量，避免抖动误标。
func (s *AccountPoolService) reapStaleInstances() {
	if s.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	instances, err := s.store.Adapters().ListInstances(ctx, "")
	if err != nil {
		log.Printf("⚠️  reaper: failed to list instances: %v", err)
		return
	}
	const staleAfter = 60 * time.Second
	now := time.Now().UTC()
	for _, inst := range instances {
		if inst.Status == domain.StatusStale {
			continue
		}
		if inst.LastHeartbeatAt == nil || now.Sub(*inst.LastHeartbeatAt) > staleAfter {
			if err := s.store.Adapters().SetInstanceStatus(ctx, inst.ID, domain.StatusStale); err != nil {
				log.Printf("⚠️  reaper: failed to mark instance %s stale: %v", inst.ID, err)
				continue
			}
			log.Printf("💀 Instance %s marked stale (no heartbeat)", inst.ID)
		}
	}
}

// 水位检查（定时任务）
func (s *AccountPoolService) checkWatermark() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sourceID, source := range s.sources {
		healthy := s.countHealthyAccountsLocked(sourceID)
		source.HealthyAccounts = healthy
		source.LastCheckAt = time.Now()

		// 低于低水位 → 自动补号。
		// 关键：同一 source 同时只允许一个补号请求在途（supplying），否则在慢速 OAuth 期间
		// 每个 ticker 周期都会重复触发，多个 goroutine 叠加导致严重超额补号。
		if healthy < int(source.Watermark.LowWatermark) {
			if source.supplying {
				continue
			}
			max := int(source.Watermark.MaxAccounts)
			if max > 0 && healthy >= max {
				continue
			}
			needed := int(source.Watermark.SupplyBatchSize)
			if max > 0 {
				if remaining := max - healthy; needed > remaining {
					needed = remaining
				}
			}
			if needed <= 0 {
				continue
			}
			source.supplying = true
			log.Printf("⚠️  Source %s below watermark: %d/%d, requesting %d accounts",
				sourceID, healthy, source.Watermark.LowWatermark, needed)

			go s.requestSupply(source, needed, "low_watermark")
		}

		// 高于高水位 → 清理过期/不健康账号（只清理已过期/不健康的，不会杀掉健康账号）
		if healthy > int(source.Watermark.HighWatermark) {
			if removed := s.cleanupAccountsLocked(sourceID); removed > 0 {
				log.Printf("🗑️  Source %s above watermark: %d/%d, cleaned up %d expired/unhealthy accounts",
					sourceID, healthy, source.Watermark.HighWatermark, removed)
			}
		}
	}
}

// 请求 adapter 补充账号
func (s *AccountPoolService) requestSupply(source *AccountSource, count int, reason string) {
	// 无论成功/失败/panic，结束后都要清除补号在途标记，否则该 source 将永远不再补号
	defer s.markSupplyDone(source)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := source.Client.SupplyAccounts(ctx, &pb.SupplyAccountsRequest{
		SourceId: source.SourceID,
		Count:    int32(count),
		Reason:   reason,
	})

	if err != nil {
		log.Printf("❌ Failed to request supply from %s: %v", source.SourceID, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, acc := range resp.Accounts {
		s.addAccountLocked(source.SourceID, acc)
	}
	s.persistAccountsLocked(ctx, source.SourceID, resp.Accounts)

	source.TotalAccounts += int(resp.SuppliedCount)
	source.HealthyAccounts = s.countHealthyAccountsLocked(source.SourceID)

	log.Printf("✅ Supplied %d accounts from %s: %s",
		resp.SuppliedCount, source.SourceID, resp.Message)
}

// markSupplyDone 清除 source 的补号在途标记。
func (s *AccountPoolService) markSupplyDone(source *AccountSource) {
	s.mu.Lock()
	source.supplying = false
	s.mu.Unlock()
}

// Helper methods (must hold lock)

func (s *AccountPoolService) addAccountLocked(sourceID string, accInfo *pb.AccountInfo) {
	acc := &Account{
		AccountID:   accInfo.AccountId,
		SourceID:    sourceID,
		Provider:    s.sources[sourceID].Provider,
		Credentials: accInfo.Credentials,
		Metadata:    accInfo.Metadata,
		ExpiresAt:   time.Unix(accInfo.ExpiresAt, 0),
		Quota:       accInfo.Quota,
		Health:      "healthy",
	}
	s.accounts[accInfo.AccountId] = acc
}

func (s *AccountPoolService) countHealthyAccountsLocked(sourceID string) int {
	count := 0
	for _, acc := range s.accounts {
		if acc.SourceID == sourceID && acc.Health == "healthy" && !acc.InUse && time.Now().Before(acc.ExpiresAt) {
			count++
		}
	}
	return count
}

func (s *AccountPoolService) filterByProviderLocked(provider string) []*Account {
	var result []*Account
	for _, acc := range s.accounts {
		if acc.Provider == provider {
			result = append(result, acc)
		}
	}
	return result
}

func (s *AccountPoolService) applyFiltersLocked(accounts []*Account, filters map[string]string) []*Account {
	if len(filters) == 0 {
		return accounts
	}

	var result []*Account
	for _, acc := range accounts {
		match := true
		for key, value := range filters {
			if acc.Metadata[key] != value {
				match = false
				break
			}
		}
		if match {
			result = append(result, acc)
		}
	}
	return result
}

func (s *AccountPoolService) selectBestAccountLocked(candidates []*Account) *Account {
	var best *Account
	for _, acc := range candidates {
		if acc.Health != "healthy" || acc.InUse || time.Now().After(acc.ExpiresAt) {
			continue
		}

		if best == nil || acc.UsageCount < best.UsageCount {
			best = acc
		}
	}
	return best
}

func (s *AccountPoolService) cleanupAccountsLocked(sourceID string) int {
	var toDelete []string
	for id, acc := range s.accounts {
		if acc.SourceID == sourceID {
			// 删除过期或不健康的账号
			if time.Now().After(acc.ExpiresAt) || acc.Health != "healthy" {
				toDelete = append(toDelete, id)
			}
		}
	}

	for _, id := range toDelete {
		delete(s.accounts, id)
		log.Printf("🗑️  Cleaned up account %s", id)
	}
	return len(toDelete)
}

func (s *AccountPoolService) persistSourceLocked(ctx context.Context, source *AccountSource) {
	if s.store == nil || source == nil {
		return
	}
	now := time.Now().UTC()
	if err := s.store.Accounts().UpsertSource(ctx, domain.AccountSource{
		SourceID:     source.SourceID,
		Provider:     source.Provider,
		CallbackAddr: source.CallbackAddr,
		Capabilities: jsonString(source.Capabilities, "[]"),
		Watermark:    jsonString(source.Watermark, "{}"),
		Status:       domain.StatusActive,
		LastSeenAt:   &now,
	}); err != nil {
		log.Printf("⚠️  Failed to persist account source %s: %v", source.SourceID, err)
	}
	if err := s.store.Adapters().UpsertAdapter(ctx, domain.Adapter{
		ID:     source.Provider,
		Name:   source.Provider,
		Type:   source.Provider,
		Status: domain.StatusActive,
	}); err != nil {
		log.Printf("⚠️  Failed to persist adapter %s: %v", source.Provider, err)
	}
	if err := s.store.Adapters().UpsertInstance(ctx, domain.AdapterInstance{
		ID:              source.SourceID,
		AdapterID:       source.Provider,
		Provider:        source.Provider,
		CallbackAddr:    source.CallbackAddr,
		Capabilities:    jsonString(source.Capabilities, "[]"),
		Metadata:        "{}",
		Status:          domain.StatusActive,
		LastHeartbeatAt: &now,
	}); err != nil {
		log.Printf("⚠️  Failed to persist adapter instance %s: %v", source.SourceID, err)
	}
}

func (s *AccountPoolService) persistAccountsLocked(ctx context.Context, sourceID string, infos []*pb.AccountInfo) {
	if s.store == nil || len(infos) == 0 {
		return
	}
	accounts := make([]domain.Account, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		source := s.sources[sourceID]
		if source == nil {
			continue
		}
		accounts = append(accounts, domain.Account{
			AccountID:   info.GetAccountId(),
			SourceID:    sourceID,
			Provider:    source.Provider,
			Credentials: jsonString(info.GetCredentials(), "{}"),
			Metadata:    jsonString(info.GetMetadata(), "{}"),
			ExpiresAt:   time.Unix(info.GetExpiresAt(), 0).UTC(),
			Health:      domain.StatusHealthy,
		})
	}
	if err := s.store.Accounts().UpsertAccounts(ctx, accounts); err != nil {
		log.Printf("⚠️  Failed to persist %d accounts for source %s: %v", len(accounts), sourceID, err)
	}
}

func (s *AccountPoolService) persistAcquiredLocked(ctx context.Context, acc *Account, sessionID string) {
	if s.store == nil || acc == nil {
		return
	}
	accounts := []domain.Account{{
		AccountID:   acc.AccountID,
		SourceID:    acc.SourceID,
		Provider:    acc.Provider,
		Credentials: jsonString(acc.Credentials, "{}"),
		Metadata:    jsonString(acc.Metadata, "{}"),
		ExpiresAt:   acc.ExpiresAt,
		Health:      acc.Health,
	}}
	if err := s.store.Accounts().UpsertAccounts(ctx, accounts); err != nil {
		log.Printf("⚠️  Failed to persist account before acquire %s: %v", acc.AccountID, err)
		return
	}
	if _, _, err := s.store.Accounts().AcquireAccountByID(ctx, acc.AccountID, sessionID, time.Now().UTC()); err != nil {
		log.Printf("⚠️  Failed to persist account acquire %s: %v", acc.AccountID, err)
	}
}

func jsonString(v interface{}, fallback string) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fallback
	}
	return string(raw)
}
