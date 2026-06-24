package proxy

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

// transportCache reuses one *http.Transport per proxy URL so we don't rebuild
// the dialer/TLS state on every request. Keyed by the canonical proxy URL.
var transportCache sync.Map // string -> *http.Transport

// TransportFor returns a cached *http.Transport that routes through proxyURL.
// An empty proxyURL returns http.DefaultTransport (no proxy).
// An unparseable URL also falls back to DefaultTransport.
func TransportFor(proxyURL string) http.RoundTripper {
	if proxyURL == "" {
		return http.DefaultTransport
	}
	if v, ok := transportCache.Load(proxyURL); ok {
		return v.(*http.Transport)
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return http.DefaultTransport
	}
	tr := &http.Transport{
		Proxy:               http.ProxyURL(u),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	actual, _ := transportCache.LoadOrStore(proxyURL, tr)
	return actual.(*http.Transport)
}

// ClientFor returns an *http.Client configured with the given proxy transport
// and timeout. Reuses the cached transport.
func ClientFor(proxyURL string, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: TransportFor(proxyURL),
		Timeout:   timeout,
	}
}

// EmbedCredentials returns a proxy URL with username/password embedded as
// userinfo (scheme://user:pass@host:port). http.Transport honors this for both
// HTTP CONNECT (Proxy-Authorization) and SOCKS5 auth. Empty user/pass returns
// rawURL unchanged; an unparseable rawURL is returned as-is.
func EmbedCredentials(rawURL, username, password string) string {
	if username == "" && password == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if password == "" {
		u.User = url.User(username)
	} else {
		u.User = url.UserPassword(username, password)
	}
	return u.String()
}
