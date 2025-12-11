package service

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsCollector collects service metrics
type MetricsCollector struct {
	registrationAttempts prometheus.Counter
	registrationSuccess  prometheus.Counter
	registrationFailure  *prometheus.CounterVec
	registrationDuration *prometheus.HistogramVec
	accountsByStatus     *prometheus.GaugeVec
	proxyRequests        prometheus.Counter
	smsRequests          prometheus.Counter
	captchaDetected      prometheus.Counter
	manualIntervention   *prometheus.CounterVec
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		registrationAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "max_service",
			Name:      "registration_attempts_total",
			Help:      "Total number of registration attempts",
		}),
		registrationSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "max_service",
			Name:      "registration_success_total",
			Help:      "Total number of successful registrations",
		}),
		registrationFailure: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "max_service",
				Name:      "registration_failure_total",
				Help:      "Total number of failed registrations by step",
			},
			[]string{"step"},
		),
		registrationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "max_service",
				Name:      "registration_duration_seconds",
				Help:      "Registration duration by step",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"step"},
		),
		accountsByStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "max_service",
				Name:      "accounts_by_status",
				Help:      "Number of accounts by status",
			},
			[]string{"status"},
		),
		proxyRequests: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "max_service",
			Name:      "proxy_requests_total",
			Help:      "Total number of proxy requests",
		}),
		smsRequests: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "max_service",
			Name:      "sms_requests_total",
			Help:      "Total number of SMS requests",
		}),
		captchaDetected: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "max_service",
			Name:      "captcha_detected_total",
			Help:      "Total number of CAPTCHAs detected",
		}),
		manualIntervention: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "max_service",
				Name:      "manual_intervention_total",
				Help:      "Total number of manual interventions by reason",
			},
			[]string{"reason"},
		),
	}
}

// IncrementRegistrationAttempts increments registration attempts counter
func (m *MetricsCollector) IncrementRegistrationAttempts() {
	m.registrationAttempts.Inc()
}

// IncrementRegistrationSuccess increments successful registrations counter
func (m *MetricsCollector) IncrementRegistrationSuccess() {
	m.registrationSuccess.Inc()
}

// IncrementRegistrationFailure increments failed registrations counter
func (m *MetricsCollector) IncrementRegistrationFailure(step string) {
	m.registrationFailure.WithLabelValues(step).Inc()
}

// RecordStepDuration records the duration of a registration step
func (m *MetricsCollector) RecordStepDuration(step string, duration time.Duration) {
	m.registrationDuration.WithLabelValues(step).Observe(duration.Seconds())
}

// UpdateAccountsByStatus updates the gauge for accounts by status
func (m *MetricsCollector) UpdateAccountsByStatus(status string, count float64) {
	m.accountsByStatus.WithLabelValues(status).Set(count)
}

// IncrementProxyRequests increments proxy requests counter
func (m *MetricsCollector) IncrementProxyRequests() {
	m.proxyRequests.Inc()
}

// IncrementSMSRequests increments SMS requests counter
func (m *MetricsCollector) IncrementSMSRequests() {
	m.smsRequests.Inc()
}

// IncrementCaptchaDetected increments CAPTCHA detected counter
func (m *MetricsCollector) IncrementCaptchaDetected() {
	m.captchaDetected.Inc()
}

// IncrementManualIntervention increments manual intervention counter
func (m *MetricsCollector) IncrementManualIntervention(reason string) {
	m.manualIntervention.WithLabelValues(reason).Inc()
}
