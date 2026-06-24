package telemetry

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var defaultMetrics = NewMetrics()

var defaultBuckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

type Metrics struct {
	mu       sync.RWMutex
	requests map[requestMetricKey]*histogram
	spans    map[spanMetricKey]*histogram
	ttfb     map[ttfbMetricKey]*histogram
	events   map[eventMetricKey]int64
}

type requestMetricKey struct {
	Method    string
	Path      string
	Status    string
	Protocol  string
	RouteMode string
}

type spanMetricKey struct {
	Span     string
	Protocol string
	Result   string
}

type ttfbMetricKey struct {
	Event    string
	Protocol string
}

type eventMetricKey struct {
	Event    string
	Protocol string
	Result   string
}

type histogram struct {
	buckets []uint64
	count   uint64
	sum     float64
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests: make(map[requestMetricKey]*histogram),
		spans:    make(map[spanMetricKey]*histogram),
		ttfb:     make(map[ttfbMetricKey]*histogram),
		events:   make(map[eventMetricKey]int64),
	}
}

func MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(200, "text/plain; version=0.0.4; charset=utf-8", []byte(defaultMetrics.RenderPrometheus()))
	}
}

func ObserveRequest(method, path string, status int, latencyMs int64, fields map[string]interface{}) {
	defaultMetrics.ObserveRequest(method, path, status, latencyMs, fields)
}

func ObserveSpan(name string, durationMs int64, fields []interface{}) {
	defaultMetrics.ObserveSpan(name, durationMs, fields)
}

func ObserveEvent(name string, fields []interface{}) {
	defaultMetrics.ObserveEvent(name, fields)
}

func (m *Metrics) ObserveRequest(method, path string, status int, latencyMs int64, fields map[string]interface{}) {
	if m == nil {
		return
	}
	key := requestMetricKey{
		Method:    method,
		Path:      path,
		Status:    strconv.Itoa(status),
		Protocol:  stringFromMap(fields, "protocol"),
		RouteMode: stringFromMap(fields, "route_mode"),
	}
	if key.Path == "" {
		key.Path = "unknown"
	}
	if key.Protocol == "" {
		key.Protocol = "unknown"
	}
	if key.RouteMode == "" {
		key.RouteMode = "unknown"
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	observeHistogram(m.requests, key, float64(latencyMs))
}

func (m *Metrics) ObserveSpan(name string, durationMs int64, fields []interface{}) {
	if m == nil || name == "" {
		return
	}
	key := spanMetricKey{
		Span:     name,
		Protocol: stringFromFields(fields, "protocol"),
		Result:   spanResult(fields),
	}
	if key.Protocol == "" {
		key.Protocol = "unknown"
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	observeHistogram(m.spans, key, float64(durationMs))
}

func (m *Metrics) ObserveEvent(name string, fields []interface{}) {
	if m == nil || name == "" {
		return
	}
	key := eventMetricKey{
		Event:    name,
		Protocol: stringFromFields(fields, "protocol"),
		Result:   stringFromFields(fields, "result"),
	}
	if key.Protocol == "" {
		key.Protocol = "unknown"
	}
	if key.Result == "" {
		key.Result = "unknown"
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[key]++

	ttfb, ok := int64FromFields(fields, "ttfb_ms")
	if !ok {
		return
	}
	observeHistogram(m.ttfb, ttfbMetricKey{Event: name, Protocol: key.Protocol}, float64(ttfb))
}

func (m *Metrics) RenderPrometheus() string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var b bytes.Buffer
	b.WriteString("# HELP polyglot_http_requests_total HTTP requests by method, route, status, protocol, and route mode.\n")
	b.WriteString("# TYPE polyglot_http_requests_total counter\n")
	for _, key := range sortedRequestKeys(m.requests) {
		fmt.Fprintf(&b, "polyglot_http_requests_total{%s} %d\n", formatLabels(requestLabels(key)), m.requests[key].count)
	}

	writeHistogramHeader(&b, "polyglot_http_request_latency_ms", "HTTP request latency in milliseconds.")
	for _, key := range sortedRequestKeys(m.requests) {
		writeHistogram(&b, "polyglot_http_request_latency_ms", requestLabels(key), m.requests[key])
	}

	b.WriteString("# HELP polyglot_spans_total Internal operation spans by name, protocol, and result.\n")
	b.WriteString("# TYPE polyglot_spans_total counter\n")
	for _, key := range sortedSpanKeys(m.spans) {
		fmt.Fprintf(&b, "polyglot_spans_total{%s} %d\n", formatLabels(spanLabels(key)), m.spans[key].count)
	}

	writeHistogramHeader(&b, "polyglot_span_duration_ms", "Internal operation span duration in milliseconds.")
	for _, key := range sortedSpanKeys(m.spans) {
		writeHistogram(&b, "polyglot_span_duration_ms", spanLabels(key), m.spans[key])
	}

	writeHistogramHeader(&b, "polyglot_adapter_ttfb_ms", "Adapter first response or first event latency in milliseconds.")
	for _, key := range sortedTTFBKeys(m.ttfb) {
		writeHistogram(&b, "polyglot_adapter_ttfb_ms", ttfbLabels(key), m.ttfb[key])
	}

	b.WriteString("# HELP polyglot_events_total Telemetry events by name, protocol, and result.\n")
	b.WriteString("# TYPE polyglot_events_total counter\n")
	for _, key := range sortedEventKeys(m.events) {
		fmt.Fprintf(&b, "polyglot_events_total{%s} %d\n", formatLabels(eventLabels(key)), m.events[key])
	}

	return b.String()
}

func (m *Metrics) Reset() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = make(map[requestMetricKey]*histogram)
	m.spans = make(map[spanMetricKey]*histogram)
	m.ttfb = make(map[ttfbMetricKey]*histogram)
	m.events = make(map[eventMetricKey]int64)
}

func observeHistogram[K comparable](values map[K]*histogram, key K, value float64) {
	h := values[key]
	if h == nil {
		h = &histogram{buckets: make([]uint64, len(defaultBuckets))}
		values[key] = h
	}
	h.count++
	h.sum += value
	for i, bucket := range defaultBuckets {
		if value <= bucket {
			h.buckets[i]++
			return
		}
	}
}

func writeHistogramHeader(b *bytes.Buffer, name, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s histogram\n", name)
}

func writeHistogram(b *bytes.Buffer, name string, labels []label, h *histogram) {
	if h == nil {
		return
	}
	cumulative := uint64(0)
	for i, bucketCount := range h.buckets {
		cumulative += bucketCount
		labelsWithBucket := append(append([]label{}, labels...), label{name: "le", value: formatBucket(defaultBuckets[i])})
		fmt.Fprintf(b, "%s_bucket{%s} %d\n", name, formatLabels(labelsWithBucket), cumulative)
	}
	labelsWithInf := append(append([]label{}, labels...), label{name: "le", value: "+Inf"})
	fmt.Fprintf(b, "%s_bucket{%s} %d\n", name, formatLabels(labelsWithInf), h.count)
	fmt.Fprintf(b, "%s_sum{%s} %.0f\n", name, formatLabels(labels), h.sum)
	fmt.Fprintf(b, "%s_count{%s} %d\n", name, formatLabels(labels), h.count)
}

type label struct {
	name  string
	value string
}

func requestLabels(key requestMetricKey) []label {
	return []label{
		{name: "method", value: key.Method},
		{name: "path", value: key.Path},
		{name: "status", value: key.Status},
		{name: "protocol", value: key.Protocol},
		{name: "route_mode", value: key.RouteMode},
	}
}

func spanLabels(key spanMetricKey) []label {
	return []label{
		{name: "span", value: key.Span},
		{name: "protocol", value: key.Protocol},
		{name: "result", value: key.Result},
	}
}

func ttfbLabels(key ttfbMetricKey) []label {
	return []label{
		{name: "event", value: key.Event},
		{name: "protocol", value: key.Protocol},
	}
}

func eventLabels(key eventMetricKey) []label {
	return []label{
		{name: "event", value: key.Event},
		{name: "protocol", value: key.Protocol},
		{name: "result", value: key.Result},
	}
}

func formatLabels(labels []label) string {
	parts := make([]string, len(labels))
	for i, label := range labels {
		parts[i] = label.name + `="` + escapeLabel(label.value) + `"`
	}
	return strings.Join(parts, ",")
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func formatBucket(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func spanResult(fields []interface{}) string {
	if stringFromFields(fields, "error") != "" {
		return "error"
	}
	return "ok"
}

func stringFromMap(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	return valueToString(values[key])
}

func stringFromFields(fields []interface{}, key string) string {
	for i := 0; i+1 < len(fields); i += 2 {
		if fieldKey, ok := fields[i].(string); ok && fieldKey == key {
			return valueToString(fields[i+1])
		}
	}
	return ""
}

func int64FromFields(fields []interface{}, key string) (int64, bool) {
	for i := 0; i+1 < len(fields); i += 2 {
		if fieldKey, ok := fields[i].(string); ok && fieldKey == key {
			return valueToInt64(fields[i+1])
		}
	}
	return 0, false
}

func valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func valueToInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func sortedRequestKeys(values map[requestMetricKey]*histogram) []requestMetricKey {
	keys := make([]requestMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprintf("%+v", keys[i]) < fmt.Sprintf("%+v", keys[j]) })
	return keys
}

func sortedSpanKeys(values map[spanMetricKey]*histogram) []spanMetricKey {
	keys := make([]spanMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprintf("%+v", keys[i]) < fmt.Sprintf("%+v", keys[j]) })
	return keys
}

func sortedTTFBKeys(values map[ttfbMetricKey]*histogram) []ttfbMetricKey {
	keys := make([]ttfbMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprintf("%+v", keys[i]) < fmt.Sprintf("%+v", keys[j]) })
	return keys
}

func sortedEventKeys(values map[eventMetricKey]int64) []eventMetricKey {
	keys := make([]eventMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprintf("%+v", keys[i]) < fmt.Sprintf("%+v", keys[j]) })
	return keys
}
