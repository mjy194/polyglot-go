package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"polyglot/internal/adapter"
	"polyglot/internal/server/middleware"
	"polyglot/internal/telemetry"
	pb "polyglot/proto/adapter"
)

func (s *Server) nativeAdapter(c *gin.Context, protocol, endpoint string) (adapter.NativeProcessor, bool) {
	// DB-routed adapter-mode provider takes precedence over the legacy config string.
	name := s.config.Backend.Provider
	if prov, ok := s.routeProviderCached(c, protocol); ok && prov.Mode == "adapter" {
		name = prov.Adapter
	}
	client, ok := s.accountService.NativeAdapterClient(name, protocol, endpoint)
	if !ok {
		return nil, false
	}
	return adapter.NewNativeProcessor(client), true
}

func (s *Server) withNative(protocol, endpoint string, fallback gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		processor, ok := s.nativeAdapter(c, protocol, endpoint)
		if !ok {
			telemetry.SetField(c, "route_mode", "universal")
			telemetry.SetField(c, "protocol", protocol)
			telemetry.SetField(c, "endpoint", endpoint)
			fallback(c)
			return
		}
		telemetry.SetField(c, "route_mode", "native")
		telemetry.SetField(c, "protocol", protocol)
		telemetry.SetField(c, "endpoint", endpoint)
		proxyURL := ""
		if s.proxyResolver != nil {
			if prov, ok := s.routeProviderCached(c, protocol); ok && prov.Mode == "adapter" {
				proxyURL, _ = s.proxyResolver.ResolveForProvider(c.Request.Context(), prov)
			} else {
				if u, err := s.proxyResolver.ResolveForName(c.Request.Context(), s.config.Backend.Provider); err == nil {
					proxyURL = u
				}
			}
		}
		if err := serveNative(c, processor, protocol, endpoint, proxyURL); err != nil && !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":    "native_adapter_error",
				},
			})
		}
		c.Abort()
	}
}

func serveNative(c *gin.Context, processor adapter.NativeProcessor, protocol, endpoint, proxyURL string) error {
	readSpan := telemetry.Start(c, "native.read_body", "protocol", protocol, "endpoint", endpoint)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		readSpan.EndError(err)
		return fmt.Errorf("read native request body: %w", err)
	}
	readSpan.End("request_bytes", len(body))

	req := &pb.NativeRequest{
		RequestId: uuid.New().String(),
		Protocol:  protocol,
		Endpoint:  endpoint,
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Query:     c.Request.URL.RawQuery,
		Headers:   singleValueHeaders(c.Request.Header),
		Body:      body,
		Stream:    nativeRequestIsStream(protocol, c, body),
		Context: &pb.RequestContext{
			UserId:   contextString(c, middleware.ContextUserID),
			ProxyUrl: proxyURL,
			Metadata: map[string]string{
				"api_key_id": contextString(c, middleware.ContextAPIKeyID),
			},
		},
	}

	flusher, _ := c.Writer.(http.Flusher)
	wroteHeader := false
	firstWrite := true
	responseBytes := 0
	writeHeader := func(resp *pb.NativeResponse) {
		if wroteHeader {
			return
		}
		for key, value := range resp.GetHeaders() {
			if value != "" {
				c.Writer.Header().Set(key, value)
			}
		}
		status := int(resp.GetStatusCode())
		if status == 0 {
			status = http.StatusOK
		}
		c.Writer.WriteHeader(status)
		wroteHeader = true
	}

	adapterStart := time.Now()
	adapterSpan := telemetry.Start(c, "native.adapter_stream", "protocol", protocol, "endpoint", endpoint, "stream", req.Stream)
	err = processor.ProcessNative(c.Request.Context(), req, func(resp *pb.NativeResponse) error {
		if nativeErr := resp.GetError(); nativeErr != nil {
			return errors.New(nativeErr.GetMessage())
		}
		if firstWrite {
			telemetry.Event(c, "native.first_response", "ttfb_ms", time.Since(adapterStart).Milliseconds())
			firstWrite = false
		}
		writeHeader(resp)
		if len(resp.GetBody()) > 0 {
			if _, err := c.Writer.Write(resp.GetBody()); err != nil {
				return err
			}
			responseBytes += len(resp.GetBody())
			if flusher != nil {
				flusher.Flush()
			}
		}
		return nil
	})
	if err != nil {
		adapterSpan.EndError(err, "response_bytes", responseBytes)
		return err
	}
	adapterSpan.End("response_bytes", responseBytes)
	if !wroteHeader {
		c.Writer.WriteHeader(http.StatusNoContent)
		c.Writer.WriteHeaderNow()
	}
	return nil
}

func contextString(c *gin.Context, key string) string {
	raw, ok := c.Get(key)
	if !ok {
		return ""
	}
	value, _ := raw.(string)
	return value
}

func singleValueHeaders(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}

func nativeRequestIsStream(protocol string, c *gin.Context, body []byte) bool {
	if protocol == "gemini" && c.Query("alt") == "sse" {
		return true
	}
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		return true
	}
	var payload struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &payload) == nil && payload.Stream
}
