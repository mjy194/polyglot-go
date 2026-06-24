package passthrough

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"polyglot/internal/config"
	"polyglot/internal/proxy"
)

const (
	ProtocolAnthropic = "anthropic"
	ProtocolOpenAI    = "openai"
	ProtocolResponses = "responses"
	ProtocolGemini    = "gemini"
)

// ProxyResolver resolves outbound proxy candidates for a protocol from the
// provider↔proxy association store. Implemented by the server using data.Store.
type ProxyResolver interface {
	// Resolve returns proxy candidates and the provider's selection strategy.
	// Empty candidates (no error) means "no proxy configured".
	Resolve(ctx context.Context, protocol string) (candidates []proxy.ResolvedProxy, strategy string, err error)
}

// Proxy forwards native provider protocol requests without converting through
// the universal adapter format.
type Proxy struct {
	enabled   bool
	cfg       config.PassthroughConfig
	client    *http.Client // default (no-proxy) client
	timeout   time.Duration
	resolver  ProxyResolver
	selectors sync.Map // "protocol\x00strategy" -> proxy.Selector
}

// New creates a passthrough proxy. It is inert unless cfg.Enabled is true.
// resolver may be nil to disable programmatic proxy support.
func New(cfg config.PassthroughConfig, resolver ProxyResolver) *Proxy {
	timeout := defaultTimeout(cfg)
	return &Proxy{
		enabled:  cfg.Enabled,
		cfg:      cfg,
		timeout:  timeout,
		resolver: resolver,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// selectorFor returns a cached Selector for the protocol/strategy pair.
func (p *Proxy) selectorFor(protocol, strategy string) proxy.Selector {
	key := protocol + "\x00" + strategy
	if v, ok := p.selectors.Load(key); ok {
		return v.(proxy.Selector)
	}
	sel := proxy.NewSelector(strategy)
	actual, _ := p.selectors.LoadOrStore(key, sel)
	return actual.(proxy.Selector)
}

// Enabled reports whether a protocol has a usable direct upstream.
func (p *Proxy) Enabled(protocol string) bool {
	if p == nil {
		return false
	}
	_, ok := p.resolve(protocol)
	return ok
}

// ServeHTTP forwards the request to the configured upstream and copies the
// provider response back byte-for-byte at the HTTP boundary.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, protocol string) error {
	upstream, ok := p.resolve(protocol)
	if !ok {
		return fmt.Errorf("passthrough upstream not configured for %s", protocol)
	}

	target, err := targetURL(upstream.Endpoint(), r.URL)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		return err
	}
	copyRequestHeaders(req.Header, r.Header)
	applyConfiguredHeaders(req.Header, upstream.Headers)
	applyAuth(req, protocol, upstream)

	// Resolve an outbound proxy for this protocol's provider (if any).
	client := p.client
	var pickedID string
	var pickedSel proxy.Selector
	if p.resolver != nil {
		if cands, strategy, rerr := p.resolver.Resolve(r.Context(), protocol); rerr == nil && len(cands) > 0 {
			sel := p.selectorFor(protocol, strategy)
			if pick, ok := sel.Pick(cands); ok {
				client = proxy.ClientFor(pick.URL, p.timeout)
				pickedID, pickedSel = pick.ID, sel
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if pickedSel != nil {
			pickedSel.MarkResult(pickedID, false) // advance failover for next request
		}
		return err
	}
	if pickedSel != nil {
		pickedSel.MarkResult(pickedID, true)
	}
	defer resp.Body.Close()

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

func (p *Proxy) resolve(protocol string) (config.UpstreamConfig, bool) {
	if !p.enabled {
		return config.UpstreamConfig{}, false
	}
	if p.cfg.Upstreams != nil {
		if upstream, ok := p.cfg.Upstreams[protocol]; ok && upstream.Endpoint() != "" {
			return upstream, true
		}
		if protocol == ProtocolResponses {
			if upstream, ok := p.cfg.Upstreams[ProtocolOpenAI]; ok && upstream.Endpoint() != "" {
				return upstream, true
			}
		}
	}
	if p.cfg.Default.Endpoint() != "" {
		return p.cfg.Default, true
	}
	return config.UpstreamConfig{}, false
}

func defaultTimeout(cfg config.PassthroughConfig) time.Duration {
	seconds := cfg.Default.TimeoutSeconds
	for _, upstream := range cfg.Upstreams {
		if upstream.TimeoutSeconds > seconds {
			seconds = upstream.TimeoutSeconds
		}
	}
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func targetURL(baseURL string, incoming *url.URL) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid passthrough upstream url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid passthrough upstream url %q: scheme and host are required", baseURL)
	}

	requestPath := normalizeGatewayPath(incoming.EscapedPath())
	basePath := strings.TrimRight(base.EscapedPath(), "/")
	if basePath == "" || requestPath == basePath || strings.HasPrefix(requestPath, basePath+"/") {
		base.Path = requestPath
	} else {
		base.Path = joinURLPath(basePath, requestPath)
	}
	base.RawQuery = mergeRawQuery(base.RawQuery, incoming.RawQuery)
	return base.String(), nil
}

func normalizeGatewayPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/"):
		return strings.TrimPrefix(path, "/api")
	case strings.HasPrefix(path, "/api/v1beta/"):
		return strings.TrimPrefix(path, "/api")
	default:
		return path
	}
}

func joinURLPath(basePath, requestPath string) string {
	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(requestPath, "/")
}

func mergeRawQuery(baseQuery, requestQuery string) string {
	if baseQuery == "" {
		return requestQuery
	}
	if requestQuery == "" {
		return baseQuery
	}
	return baseQuery + "&" + requestQuery
}

func copyRequestHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) || strings.EqualFold(key, "Host") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func applyConfiguredHeaders(headers http.Header, configured map[string]string) {
	for key, value := range configured {
		headers.Set(key, value)
	}
}

func applyAuth(req *http.Request, protocol string, upstream config.UpstreamConfig) {
	if upstream.APIKey == "" {
		applyProtocolDefaults(req.Header, protocol)
		return
	}

	if upstream.APIKeyHeader != "" {
		req.Header.Set(upstream.APIKeyHeader, upstream.APIKey)
		applyProtocolDefaults(req.Header, protocol)
		return
	}

	authType := upstream.AuthType
	if authType == "" {
		authType = defaultAuthType(protocol)
	}

	switch strings.ToLower(authType) {
	case "none":
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	case "x-api-key":
		req.Header.Set("x-api-key", upstream.APIKey)
	case "x-goog-api-key":
		req.Header.Set("x-goog-api-key", upstream.APIKey)
	case "query":
		queryName := upstream.APIKeyQuery
		if queryName == "" {
			queryName = "key"
		}
		q := req.URL.Query()
		q.Set(queryName, upstream.APIKey)
		req.URL.RawQuery = q.Encode()
	default:
		req.Header.Set(authType, upstream.APIKey)
	}

	applyProtocolDefaults(req.Header, protocol)
}

func defaultAuthType(protocol string) string {
	switch protocol {
	case ProtocolAnthropic:
		return "x-api-key"
	case ProtocolGemini:
		return "x-goog-api-key"
	default:
		return "bearer"
	}
}

func applyProtocolDefaults(headers http.Header, protocol string) {
	if protocol == ProtocolAnthropic && headers.Get("anthropic-version") == "" {
		headers.Set("anthropic-version", "2023-06-01")
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}
