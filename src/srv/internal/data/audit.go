package data

import (
	"context"
	"time"

	"polyglot/internal/domain"

	"gorm.io/gorm"
)

// AuditRepository persists request and usage audit trails.
type AuditRepository interface {
	RecordRequest(ctx context.Context, log domain.RequestLog) error
	ListRequestLogs(ctx context.Context, filter RequestLogFilter) ([]domain.RequestLog, error)
	ListUsageEvents(ctx context.Context, filter UsageEventFilter) ([]domain.UsageEvent, error)
	RequestStats(ctx context.Context, filter RequestLogFilter) (RequestStats, error)
	RequestStatsByProvider(ctx context.Context, from time.Time) (map[string]RequestStats, error)
	// HourlyHealthByProvider returns, per provider, 24 hourly buckets covering the
	// last 24h. Slot 0 = oldest (23h ago), slot 23 = the current hour (UTC).
	HourlyHealthByProvider(ctx context.Context, from time.Time, now time.Time) (map[string][]HealthBucket, error)
}

// HealthBucket is one hour's request health for a provider.
type HealthBucket struct {
	Total     int64 `json:"total"`
	Successes int64 `json:"successes"`
}

type RequestLogFilter struct {
	UserID   string
	Provider string
	Protocol string
	From     time.Time
	To       time.Time
	Limit    int
}

type UsageEventFilter struct {
	UserID    string
	AccountID string
	Provider  string
	Model     string
	From      time.Time
	To        time.Time
	Limit     int
}

type RequestStats struct {
	RequestsTotal    int64
	SuccessTotal     int64
	SuccessRate      float64
	AverageLatencyMs float64
}

type gormAuditRepository struct {
	db *gorm.DB
}

func NewGormAuditRepository(db *gorm.DB) AuditRepository {
	return &gormAuditRepository{db: db}
}

func (r *gormAuditRepository) RecordRequest(ctx context.Context, log domain.RequestLog) error {
	record := requestLogToRecord(log)
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *gormAuditRepository) ListRequestLogs(ctx context.Context, filter RequestLogFilter) ([]domain.RequestLog, error) {
	var records []RequestLogRecord
	q := applyRequestLogFilter(r.db.WithContext(ctx), filter).Order("created_at DESC")
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	err := q.Find(&records).Error
	return requestLogsFromRecords(records), err
}

func (r *gormAuditRepository) ListUsageEvents(ctx context.Context, filter UsageEventFilter) ([]domain.UsageEvent, error) {
	var records []UsageEventRecord
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if filter.UserID != "" {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.AccountID != "" {
		q = q.Where("account_id = ?", filter.AccountID)
	}
	if filter.Provider != "" {
		q = q.Where("provider = ?", filter.Provider)
	}
	if filter.Model != "" {
		q = q.Where("model = ?", filter.Model)
	}
	if !filter.From.IsZero() {
		q = q.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		q = q.Where("created_at <= ?", filter.To)
	}
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	err := q.Find(&records).Error
	return usageEventsFromRecords(records), err
}

func (r *gormAuditRepository) RequestStats(ctx context.Context, filter RequestLogFilter) (RequestStats, error) {
	type aggregate struct {
		RequestsTotal    int64
		SuccessTotal     int64
		AverageLatencyMs float64
	}
	var agg aggregate
	q := applyRequestLogFilter(r.db.WithContext(ctx).Model(&RequestLogRecord{}), filter)
	err := q.Select(
		"COUNT(*) AS requests_total, SUM(CASE WHEN success THEN 1 ELSE 0 END) AS success_total, COALESCE(AVG(latency_ms), 0) AS average_latency_ms",
	).Scan(&agg).Error
	if err != nil {
		return RequestStats{}, err
	}
	stats := RequestStats{
		RequestsTotal:    agg.RequestsTotal,
		SuccessTotal:     agg.SuccessTotal,
		AverageLatencyMs: agg.AverageLatencyMs,
	}
	if stats.RequestsTotal > 0 {
		stats.SuccessRate = float64(stats.SuccessTotal) / float64(stats.RequestsTotal)
	}
	return stats, nil
}

// RequestStatsByProvider aggregates request stats grouped by provider since `from`.
func (r *gormAuditRepository) RequestStatsByProvider(ctx context.Context, from time.Time) (map[string]RequestStats, error) {
	type row struct {
		Provider         string
		RequestsTotal    int64
		SuccessTotal     int64
		AverageLatencyMs float64
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&RequestLogRecord{}).
		Where("created_at >= ?", from).
		Select("provider, COUNT(*) AS requests_total, SUM(CASE WHEN success THEN 1 ELSE 0 END) AS success_total, COALESCE(AVG(latency_ms), 0) AS average_latency_ms").
		Group("provider").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]RequestStats, len(rows))
	for _, r := range rows {
		s := RequestStats{RequestsTotal: r.RequestsTotal, SuccessTotal: r.SuccessTotal, AverageLatencyMs: r.AverageLatencyMs}
		if s.RequestsTotal > 0 {
			s.SuccessRate = float64(s.SuccessTotal) / float64(s.RequestsTotal)
		}
		out[r.Provider] = s
	}
	return out, nil
}

// HourlyHealthByProvider aggregates request logs into 24 hourly buckets per provider.
func (r *gormAuditRepository) HourlyHealthByProvider(ctx context.Context, from, now time.Time) (map[string][]HealthBucket, error) {
	type row struct {
		Provider string
		HourTs   string `gorm:"column:hour_ts"`
		Total    int64
		Succ     int64
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&RequestLogRecord{}).
		Where("created_at >= ?", from).
		Select("provider, strftime('%Y-%m-%dT%H:00:00', created_at) AS hour_ts, COUNT(*) AS total, SUM(CASE WHEN success THEN 1 ELSE 0 END) AS succ").
		Group("provider, hour_ts").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	nowHour := now.UTC().Truncate(time.Hour)
	out := make(map[string][]HealthBucket)
	for _, r := range rows {
		t, err := time.ParseInLocation("2006-01-02T15:00:00", r.HourTs, time.UTC)
		if err != nil {
			continue
		}
		hoursAgo := int(nowHour.Sub(t.Truncate(time.Hour)) / time.Hour)
		if hoursAgo < 0 || hoursAgo > 23 {
			continue
		}
		slots, ok := out[r.Provider]
		if !ok {
			slots = make([]HealthBucket, 24)
			out[r.Provider] = slots
		}
		slots[23-hoursAgo] = HealthBucket{Total: r.Total, Successes: r.Succ}
	}
	return out, nil
}

func applyRequestLogFilter(q *gorm.DB, filter RequestLogFilter) *gorm.DB {
	if filter.UserID != "" {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.Provider != "" {
		q = q.Where("provider = ?", filter.Provider)
	}
	if filter.Protocol != "" {
		q = q.Where("protocol = ?", filter.Protocol)
	}
	if !filter.From.IsZero() {
		q = q.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		q = q.Where("created_at <= ?", filter.To)
	}
	return q
}
