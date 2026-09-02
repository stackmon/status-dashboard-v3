package notification

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const metricsNamespace = "notification"

// Metrics holds the worker-driven counters and histogram. Queue-depth gauges are
// exposed separately by the DB-backed statsCollector (pulled on scrape).
//
// All record* methods are nil-safe so the worker can run without metrics (tests).
type Metrics struct {
	sent           *prometheus.CounterVec // by kind
	failed         *prometheus.CounterVec // by kind
	attempts       prometheus.Counter
	staleRecovered prometheus.Counter
	duration       prometheus.Histogram
}

// NewMetrics builds the notification delivery metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		sent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "sent_total",
			Help: "Total notification emails accepted by the mail server, by kind.",
		}, []string{"kind"}),
		failed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "failed_total",
			Help: "Total notification send failures (retryable and terminal), by kind.",
		}, []string{"kind"}),
		attempts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "attempts_total",
			Help: "Total notification delivery attempts.",
		}),
		staleRecovered: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "stale_recovered_total",
			Help: "Total processing rows recovered after a lease timeout.",
		}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: metricsNamespace, Name: "delivery_duration_seconds",
			Help:    "Time to render and send one notification.",
			Buckets: prometheus.DefBuckets,
		}),
	}
}

// MustRegister registers the worker metrics on reg.
func (m *Metrics) MustRegister(reg prometheus.Registerer) {
	reg.MustRegister(m.sent, m.failed, m.attempts, m.staleRecovered, m.duration)
}

func (m *Metrics) recordSent(kind string) {
	if m == nil {
		return
	}
	m.sent.WithLabelValues(kind).Inc()
}

func (m *Metrics) recordFailed(kind string) {
	if m == nil {
		return
	}
	m.failed.WithLabelValues(kind).Inc()
}

func (m *Metrics) recordAttempt() {
	if m == nil {
		return
	}
	m.attempts.Inc()
}

func (m *Metrics) recordStaleRecovered(n int) {
	if m == nil || n <= 0 {
		return
	}
	m.staleRecovered.Add(float64(n))
}

func (m *Metrics) observeDuration(d time.Duration) {
	if m == nil {
		return
	}
	m.duration.Observe(d.Seconds())
}

// statsCollector emits queue-depth gauges by querying the outbox on each scrape,
// so they always reflect current state without per-operation bookkeeping.
type statsCollector struct {
	db             *db.DB
	staleThreshold time.Duration

	pending         *prometheus.Desc
	processing      *prometheus.Desc
	failed          *prometheus.Desc
	staleProcessing *prometheus.Desc
	retryBacklog    *prometheus.Desc
	oldestAge       *prometheus.Desc
}

// NewStatsCollector builds the DB-backed queue-depth collector.
func NewStatsCollector(database *db.DB, staleThreshold time.Duration) prometheus.Collector {
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(metricsNamespace+"_"+name, help, nil, nil)
	}
	return &statsCollector{
		db:              database,
		staleThreshold:  staleThreshold,
		pending:         desc("outbox_pending", "Outbox rows waiting to be sent."),
		processing:      desc("outbox_processing", "Outbox rows currently being sent."),
		failed:          desc("outbox_failed", "Outbox rows in the terminal failed state."),
		staleProcessing: desc("outbox_stale_processing", "Processing rows whose lease has expired."),
		retryBacklog:    desc("outbox_retry_backlog", "Pending rows waiting for a future retry."),
		oldestAge:       desc("outbox_oldest_pending_age_seconds", "Age of the oldest undelivered row."),
	}
}

func (c *statsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pending
	ch <- c.processing
	ch <- c.failed
	ch <- c.staleProcessing
	ch <- c.retryBacklog
	ch <- c.oldestAge
}

func (c *statsCollector) Collect(ch chan<- prometheus.Metric) {
	stats, err := c.db.GetNotificationStats(context.Background(), c.staleThreshold)
	if err != nil {
		return // a scrape-time DB error just omits the gauges this round
	}
	g := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v)
	}
	g(c.pending, float64(stats.Pending))
	g(c.processing, float64(stats.Processing))
	g(c.failed, float64(stats.Failed))
	g(c.staleProcessing, float64(stats.StaleProcessing))
	g(c.retryBacklog, float64(stats.RetryBacklog))
	g(c.oldestAge, stats.OldestPendingAgeSeconds)
}
