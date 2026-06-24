package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"polyglot/internal/adapter"
	"polyglot/internal/passthrough"
	"polyglot/internal/server/handler"
	"polyglot/internal/server/middleware"
	"polyglot/internal/telemetry"
)

func (s *Server) setupRoutes() {
	// 全局中间件
	s.router.Use(telemetry.Middleware(s.logger))
	s.router.Use(gin.Recovery())
	s.router.Use(middleware.CORS(s.config.Server.CORS))
	s.router.Use(middleware.RequestAudit(s.dataStore, s.config.Backend.Provider))

	// 健康检查
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "polyglot",
		})
	})
	s.router.GET("/metrics", telemetry.MetricsHandler())

	// Universal 路径解析器：按协议查 DB provider（adapter 模式用 provider.Adapter），
	// 否则落回 config.Backend.Provider。adapter 注册前返回 503。
	universalResolver := func(protocol string) func() (adapter.StreamProcessor, bool) {
		return func() (adapter.StreamProcessor, bool) {
			name := s.config.Backend.Provider
			if prov, ok := s.routeProvider(context.Background(), protocol); ok && prov.Mode == "adapter" {
				name = prov.Adapter
			}
			client, ok := s.accountService.AdapterClient(name)
			if !ok {
				return nil, false
			}
			return adapter.NewStreamProcessor(client), true
		}
	}

	anthropicHandler := s.withPassthrough(passthrough.ProtocolAnthropic,
		s.withNative(passthrough.ProtocolAnthropic, "messages", handler.AnthropicMessages(universalResolver(passthrough.ProtocolAnthropic))))
	openAIHandler := s.withPassthrough(passthrough.ProtocolOpenAI,
		s.withNative(passthrough.ProtocolOpenAI, "chat_completions", handler.OpenAIChatCompletions(universalResolver(passthrough.ProtocolOpenAI))))
	responsesHandler := s.withPassthrough(passthrough.ProtocolResponses,
		s.withNative(passthrough.ProtocolResponses, "responses", handler.Responses(universalResolver(passthrough.ProtocolResponses))))
	geminiHandler := s.withPassthrough(passthrough.ProtocolGemini,
		s.withNative(passthrough.ProtocolGemini, "generate_content", handler.GeminiGenerateContent(universalResolver(passthrough.ProtocolGemini))))
	apiKeyAuth := middleware.APIKeyAuth(s.dataStore)

	// 标准路径 /v1/messages（真实 Anthropic 客户端如 claude CLI 走此路径）
	s.router.POST("/v1/messages", apiKeyAuth, middleware.RequireScope("anthropic"), anthropicHandler)
	// 标准路径 /v1/chat/completions（真实 OpenAI 客户端如 codex CLI 走此路径）
	s.router.POST("/v1/chat/completions", apiKeyAuth, middleware.RequireScope("openai"), openAIHandler)
	// 标准路径 /v1/responses（OpenAI Responses API，codex CLI 0.140+ 仅支持此 wire）
	s.router.POST("/v1/responses", apiKeyAuth, middleware.RequireScope("responses"), responsesHandler)

	// API 路由
	api := s.router.Group("/api")
	{
		adminSession := api.Group("/admin")
		{
			adminSession.POST("/bootstrap", handler.AuthBootstrap(s.dataStore))
			adminSession.POST("/login", handler.AuthLogin(s.dataStore))
			adminSession.GET("/profile", middleware.AdminAuth(s.dataStore), handler.AuthProfile(s.dataStore))
			adminSession.POST("/logout", middleware.AdminAuth(s.dataStore), handler.AuthLogout(s.dataStore))
		}

		// AI APIs
		v1 := api.Group("/v1")
		{
			// /api/v1/messages —— 兼容旧路径
			v1.POST("/messages", apiKeyAuth, middleware.RequireScope("anthropic"), anthropicHandler)

			// OpenAI Chat Completions API（与 Anthropic 共用同一 adapter 解析器）
			v1.POST("/chat/completions", apiKeyAuth, middleware.RequireScope("openai"), openAIHandler)
			v1.POST("/responses", apiKeyAuth, middleware.RequireScope("responses"), responsesHandler)
		}

		// Gemini API (uses different versioning)
		// POST /api/v1beta/models/{model}:generateContent
		v1beta := api.Group("/v1beta")
		{
			// Gemini generateContent（与 Anthropic/OpenAI 共用同一 adapter）
			// Matches: /api/v1beta/{model}[:generateContent]
			v1beta.POST("/:model", apiKeyAuth, middleware.RequireScope("gemini"), geminiHandler)
			v1beta.POST("/models/:model", apiKeyAuth, middleware.RequireScope("gemini"), geminiHandler)
		}

		admin := api.Group("/admin")
		admin.Use(middleware.AdminAuth(s.dataStore), middleware.RequireScope("admin"))
		{
			admin.GET("/stats", handler.AdminStats(s.dataStore))
			admin.GET("/users", handler.AdminUsers(s.dataStore))
			admin.POST("/users", handler.AdminUpsertUser(s.dataStore))
			admin.GET("/roles", handler.AdminRoles(s.dataStore))
			admin.POST("/roles", handler.AdminUpsertRole(s.dataStore))
			admin.GET("/user-roles", handler.AdminUserRoles(s.dataStore))
			admin.POST("/user-roles", handler.AdminAssignRole(s.dataStore))
			admin.GET("/api-keys", handler.AdminAPIKeys(s.dataStore))
			admin.POST("/api-keys", handler.AdminUpsertAPIKey(s.dataStore))
			admin.GET("/providers", handler.AdminProviders(s.dataStore))
			admin.POST("/providers", handler.AdminUpsertProvider(s.dataStore))
			admin.GET("/providers/health", handler.AdminProviderHealth(s.dataStore))
			admin.GET("/providers/health/hourly", handler.AdminProviderHealthHourly(s.dataStore))
			admin.GET("/providers/:id/proxies", handler.AdminListProviderProxies(s.dataStore))
			admin.POST("/providers/:id/proxies", handler.AdminSetProviderProxies(s.dataStore))
			admin.GET("/providers/:id/model-mappings", handler.AdminListProviderModelMappings(s.dataStore))
			admin.POST("/providers/:id/model-mappings", handler.AdminUpsertProviderModelMapping(s.dataStore))
			admin.DELETE("/model-mappings/:id", handler.AdminDeleteModelMapping(s.dataStore))
			admin.GET("/proxies", handler.AdminProxies(s.dataStore))
			admin.POST("/proxies", handler.AdminUpsertProxy(s.dataStore))
			admin.DELETE("/proxies/:id", handler.AdminDeleteProxy(s.dataStore))
			admin.POST("/proxies/:id/test", handler.AdminTestProxy(s.dataStore))
			admin.GET("/adapters", handler.AdminAdapters(s.dataStore))
			admin.GET("/adapter-instances", handler.AdminAdapterInstances(s.dataStore))
			admin.GET("/request-logs", handler.AdminRequestLogs(s.dataStore))
			admin.GET("/usage-events", handler.AdminUsageEvents(s.dataStore))
		}
	}

	// 标准路径 /v1beta/{model}（真实 Gemini 客户端如 gemini CLI 走此路径）
	v1betaRoot := s.router.Group("/v1beta")
	v1betaRoot.POST("/:model", apiKeyAuth, middleware.RequireScope("gemini"), geminiHandler)
	v1betaRoot.POST("/models/:model", apiKeyAuth, middleware.RequireScope("gemini"), geminiHandler)
}

func (s *Server) withPassthrough(protocol string, fallback gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// DB-driven override: a passthrough-mode provider serves directly; an
		// adapter-mode provider skips passthrough and falls through to native.
		if prov, ok := s.routeProviderCached(c, protocol); ok {
			// Record the resolved DB provider name so the audit log attributes
			// the request to it (enables per-provider health).
			c.Set(middleware.ContextAuditProvider, prov.Name)
			if prov.Mode == "passthrough" {
				telemetry.SetField(c, "route_mode", "passthrough")
				telemetry.SetField(c, "protocol", protocol)
				span := telemetry.Start(c, "route.passthrough", "protocol", protocol)
				upstream := providerUpstream(prov)
				err := s.passthrough.ServeHTTPWithUpstream(c.Writer, c.Request, protocol, upstream)
				if err != nil {
					if !c.Writer.Written() {
						c.JSON(http.StatusBadGateway, gin.H{
							"error": gin.H{
								"message": err.Error(),
								"type":    "passthrough_error",
							},
						})
					}
					span.EndError(err)
				} else {
					span.End()
				}
				c.Abort()
				return
			}
			// mode == adapter → fall through to native (which uses prov.Adapter)
			fallback(c)
			return
		}

		// Legacy config-based passthrough.
		if s.passthrough != nil && s.passthrough.Enabled(protocol) {
			telemetry.SetField(c, "route_mode", "passthrough")
			telemetry.SetField(c, "protocol", protocol)
			span := telemetry.Start(c, "route.passthrough", "protocol", protocol)
			if err := s.passthrough.ServeHTTP(c.Writer, c.Request, protocol); err != nil {
				if !c.Writer.Written() {
					c.JSON(http.StatusBadGateway, gin.H{
						"error": gin.H{
							"message": err.Error(),
							"type":    "passthrough_error",
						},
					})
				}
				span.EndError(err)
			} else {
				span.End()
			}
			c.Abort()
			return
		}

		fallback(c)
	}
}
