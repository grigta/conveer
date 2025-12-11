package service

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricsCollector interface {
	IncrementAccountsTotal(status string)
	DecrementAccountsTotal(status string)
	IncrementRegistrationsTotal(result string)
	RecordRegistrationDuration(duration time.Duration)
	IncrementRetryAttempts()
	IncrementActiveRegistrations()
	DecrementActiveRegistrations()
	UpdateBrowserPoolSize(size int)
	IncrementErrorsTotal(errorType string)
	IncrementManualInterventions()
	GetTotalAccounts() int64
}

type metricsCollector struct {
	accountsTotal           *prometheus.GaugeVec
	registrationsTotal      *prometheus.CounterVec
	registrationDuration    prometheus.Histogram
	retryAttemptsTotal      prometheus.Counter
	activeRegistrations     prometheus.Gauge
	browserPoolSize         prometheus.Gauge
	errorsTotal             *prometheus.CounterVec
	manualInterventionsTotal prometheus.Counter
	totalAccountsCache      int64
}

func NewMetricsCollector() MetricsCollector {
	return &metricsCollector{
		accountsTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vk_accounts_total",
				Help: "Total number of VK accounts by status",
			},
			[]string{"status"},
		),
		registrationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vk_registrations_total",
				Help: "Total number of registration attempts by result",
			},
			[]string{"result"},
		),
		registrationDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "vk_registration_duration_seconds",
				Help:    "Registration duration in seconds",
				Buckets: prometheus.ExponentialBuckets(10, 2, 10), // 10s to ~2.8h
			},
		),
		retryAttemptsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "vk_retry_attempts_total",
				Help: "Total number of retry attempts",
			},
		),
		activeRegistrations: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "vk_active_registrations",
				Help: "Number of currently active registrations",
			},
		),
		browserPoolSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "vk_browser_pool_size",
				Help: "Current size of the browser pool",
			},
		),
		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vk_errors_total",
				Help: "Total number of errors by type",
			},
			[]string{"type"},
		),
		manualInterventionsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "vk_manual_interventions_total",
				Help: "Total number of manual intervention requests",
			},
		),
	}
}

func (m *metricsCollector) IncrementAccountsTotal(status string) {
	m.accountsTotal.WithLabelValues(status).Inc()
	m.totalAccountsCache++
}

func (m *metricsCollector) DecrementAccountsTotal(status string) {
	m.accountsTotal.WithLabelValues(status).Dec()
	m.totalAccountsCache--
}

func (m *metricsCollector) IncrementRegistrationsTotal(result string) {
	m.registrationsTotal.WithLabelValues(result).Inc()
}

func (m *metricsCollector) RecordRegistrationDuration(duration time.Duration) {
	m.registrationDuration.Observe(duration.Seconds())
}

func (m *metricsCollector) IncrementRetryAttempts() {
	m.retryAttemptsTotal.Inc()
}

func (m *metricsCollector) IncrementActiveRegistrations() {
	m.activeRegistrations.Inc()
}

func (m *metricsCollector) DecrementActiveRegistrations() {
	m.activeRegistrations.Dec()
}

func (m *metricsCollector) UpdateBrowserPoolSize(size int) {
	m.browserPoolSize.Set(float64(size))
}

func (m *metricsCollector) IncrementErrorsTotal(errorType string) {
	m.errorsTotal.WithLabelValues(errorType).Inc()
}

func (m *metricsCollector) IncrementManualInterventions() {
	m.manualInterventionsTotal.Inc()
}

func (m *metricsCollector) GetTotalAccounts() int64 {
	return m.totalAccountsCache
}