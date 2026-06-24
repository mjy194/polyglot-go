package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"polyglot/internal/account"
	"polyglot/internal/config"
	"polyglot/internal/data"
	"polyglot/internal/domain"
	"polyglot/internal/passthrough"
	"polyglot/internal/proxy"
	"polyglot/internal/storage"
	"polyglot/pkg/logger"
	pb "polyglot/proto/adapter"
)

// Server HTTP 服务器
type Server struct {
	config         *config.Config
	router         *gin.Engine
	logger         logger.Logger
	srv            *http.Server
	grpcServer     *grpc.Server
	dataStore      *data.Store
	storageServer  *storage.UiPathStorageService
	accountService *account.AccountPoolService
	passthrough    *passthrough.Proxy
	proxyResolver  *storeProxyResolver
}

// New 创建服务器实例
func New(cfg *config.Config, log logger.Logger) (*Server, error) {
	// 创建路由
	router := gin.New()

	// 创建 data store + gRPC 存储服务
	dataStore, err := data.Open(dataConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to open data store: %w", err)
	}
	storageServer := storage.NewUiPathStorageServiceWithStore(dataStore)

	// 创建账号池服务
	accountService := account.NewAccountPoolServiceWithStore(dataStore)

	passthroughCfg := cfg.Backend.Passthrough
	if cfg.Backend.Provider == "passthrough" {
		passthroughCfg.Enabled = true
	}
	proxyResolver := &storeProxyResolver{store: dataStore}
	passthroughProxy := passthrough.New(passthroughCfg, proxyResolver)

	// 创建 gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterStorageServiceServer(grpcServer, storageServer)
	pb.RegisterAccountServiceServer(grpcServer, accountService)

	s := &Server{
		config:         cfg,
		router:         router,
		logger:         log,
		grpcServer:     grpcServer,
		dataStore:      dataStore,
		storageServer:  storageServer,
		accountService: accountService,
		passthrough:    passthroughProxy,
		proxyResolver:  proxyResolver,
	}

	// 设置路由
	s.setupRoutes()

	return s, nil
}

func dataConfig(cfg *config.Config) data.Config {
	driver := cfg.Database.Driver
	dsn := cfg.Database.DSN
	if dsn == "" {
		if raw, ok := cfg.Backend.UiPath["auth_db_path"]; ok {
			if s, ok := raw.(string); ok && s != "" {
				dsn = s
			}
		}
	}
	if dsn == "" {
		dsn = "../../../data.db"
	}

	autoMigrate := true
	if cfg.Database.AutoMigrate != nil {
		autoMigrate = *cfg.Database.AutoMigrate
	}

	return data.Config{
		Driver:      driver,
		DSN:         dsn,
		AutoMigrate: autoMigrate,
	}
}

// Run 启动服务器
func (s *Server) Run() error {
	// HTTP 监听：双栈（IPv4+IPv6）
	httpLis, err := listenDualStack(s.config.Server.Host, s.config.Server.Port)
	if err != nil {
		return fmt.Errorf("failed to listen http: %w", err)
	}
	s.srv = &http.Server{
		Handler:        s.router,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// 启动 gRPC StorageService（在独立端口，双栈）
	grpcPort := 50052 // StorageService 端口
	go func() {
		lis, err := listenDualStack("", grpcPort)
		if err != nil {
			s.logger.Error("Failed to listen for gRPC", "error", err)
			return
		}
		s.logger.Info("gRPC Services starting", "port", grpcPort, "services", "StorageService,AccountService")
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server error", "error", err)
		}
	}()

	// 启动账号池水位监控
	s.accountService.Start()

	s.logger.Info("HTTP Server starting", "address", httpLis.Addr().String())

	return s.srv.Serve(httpLis)
}

// listenDualStack 在指定端口监听，支持 IPv4 与 IPv6 双栈：
//   - host 为通配（""、"0.0.0.0"、"::"）时优先绑 [::]（Linux 默认双栈，同时收 v4+v6 连接），失败回退 0.0.0.0；
//   - host 为具体 IP 时按该 IP 绑。
//
// 这样客户端无论用 127.0.0.1、::1 还是 localhost 都能连上。
func listenDualStack(host string, port int) (net.Listener, error) {
	if host == "" || host == "0.0.0.0" || host == "::" {
		if lis, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", port)); err == nil {
			return lis, nil
		}
		return net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	}
	return net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
}

// Shutdown 优雅关闭
func (s *Server) Shutdown(ctx context.Context) error {
	// 停止账号池监控
	if s.accountService != nil {
		s.accountService.Stop()
	}

	// 关闭 gRPC server
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// 关闭存储服务
	if s.storageServer != nil {
		s.storageServer.Close()
	}

	if s.dataStore != nil {
		_ = s.dataStore.Close()
	}

	return s.srv.Shutdown(ctx)
}

// storeProxyResolver resolves outbound proxy candidates for a passthrough
// protocol by mapping protocol → active provider (by Type) → its M:N proxies.
// responses falls back to the openai provider's proxies (mirroring upstream
// resolution). Returns empty candidates when no provider/proxy is configured.
type storeProxyResolver struct {
	store      *data.Store
	selectors  sync.Map // key (e.g. protocol or provider name) + strategy → proxy.Selector
}

// selectorFor returns a cached Selector keyed by (key, strategy).
func (r *storeProxyResolver) selectorFor(key, strategy string) proxy.Selector {
	k := key + "\x00" + strategy
	if v, ok := r.selectors.Load(k); ok {
		return v.(proxy.Selector)
	}
	sel := proxy.NewSelector(strategy)
	actual, _ := r.selectors.LoadOrStore(k, sel)
	return actual.(proxy.Selector)
}

// gatherCandidates returns the proxy candidates + strategy for a provider.
func (r *storeProxyResolver) gatherCandidates(ctx context.Context, prov *domain.Provider) ([]proxy.ResolvedProxy, string, error) {
	links, err := r.store.Proxies().ListProviderProxies(ctx, prov.ID)
	if err != nil {
		return nil, "", err
	}
	if len(links) == 0 {
		return nil, prov.ProxyStrategy, nil
	}
	all, err := r.store.Proxies().ListProxies(ctx)
	if err != nil {
		return nil, "", err
	}
	byID := make(map[string]domain.Proxy, len(all))
	for _, pr := range all {
		if pr.Status == domain.StatusActive {
			byID[pr.ID] = pr
		}
	}
	candidates := make([]proxy.ResolvedProxy, 0, len(links))
	for _, l := range links {
		if pr, ok := byID[l.ProxyID]; ok {
			candidates = append(candidates, proxy.ResolvedProxy{
				ID:       pr.ID,
				URL:      pr.URL,
				Priority: l.Priority,
			})
		}
	}
	return candidates, prov.ProxyStrategy, nil
}

func (r *storeProxyResolver) Resolve(ctx context.Context, protocol string) ([]proxy.ResolvedProxy, string, error) {
	matchType := protocol
	if protocol == passthrough.ProtocolResponses {
		matchType = passthrough.ProtocolOpenAI
	}

	providers, err := r.store.Providers().ListProviders(ctx)
	if err != nil {
		return nil, "", err
	}
	var prov *domain.Provider
	for i := range providers {
		if providers[i].Status == domain.StatusActive && providers[i].Type == matchType {
			prov = &providers[i]
			break
		}
	}
	if prov == nil {
		return nil, "", nil
	}
	return r.gatherCandidates(ctx, prov)
}

// ResolveForName picks one proxy URL for a provider matched by Name (used by the
// adapter/native path, which keys off config.Backend_provider). Returns "" when
// no provider or no proxies are configured. Selection honours the provider's
// strategy; failover advances across requests via the cached selector.
func (r *storeProxyResolver) ResolveForName(ctx context.Context, name string) (string, error) {
	providers, err := r.store.Providers().ListProviders(ctx)
	if err != nil {
		return "", err
	}
	var prov *domain.Provider
	for i := range providers {
		if providers[i].Status == domain.StatusActive && providers[i].Name == name {
			prov = &providers[i]
			break
		}
	}
	if prov == nil {
		return "", nil
	}
	candidates, strategy, err := r.gatherCandidates(ctx, prov)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", nil
	}
	sel := r.selectorFor("name:"+name, strategy)
	pick, ok := sel.Pick(candidates)
	if !ok {
		return "", nil
	}
	return pick.URL, nil
}
