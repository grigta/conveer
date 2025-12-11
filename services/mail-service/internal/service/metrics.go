package service

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsCollector collects metrics for the mail service
type MetricsCollector struct {
	registrationAttempts   prometheus.Counter
	registrationSuccess    prometheus.Counter
	registrationFailures   *prometheus.CounterVec
	registrationDuration   prometheus.Histogram
	stepDuration          *prometheus.HistogramVec
	proxyUsage            *prometheus.CounterVec
	smsVerifications      *prometheus.CounterVec
	captchaSolved         *prometheus.CounterVec
	manualInterventions   *prometheus.CounterVec
	sessionsActive        prometheus.Gauge
	sessionsDuration      prometheus.Histogram
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		registrationAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mail_service_registration_attempts_total",
			Help: "Total number of registration attempts",
		}),
		registrationSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mail_service_registration_success_total",
			Help: "Total number of successful registrations",
		}),
		registrationFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mail_service_registration_failures_total",
				Help: "Total number of registration failures by reason",
			},
			[]string{"reason"},
		),
		registrationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "mail_service_registration_duration_seconds",
			Help:    "Time taken to complete registration",
			Buckets: prometheus.ExponentialBuckets(10, 2, 8),
		}),
		stepDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mail_service_step_duration_seconds",
				Help:    "Time taken for each registration step",
				Buckets: prometheus.ExponentialBuckets(1, 2, 8),
			},
			[]string{"step"},
		),
		proxyUsage: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mail_service_proxy_usage_total",
				Help: "Proxy usage by type",
			},
			[]string{"type", "status"},
		),
		smsVerifications: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mail_service_sms_verifications_total",
				Help: "SMS verification attempts",
			},
			[]string{"status"},
		),
		captchaSolved: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mail_service_captcha_solved_total",
				Help: "Captcha solving attempts",
			},
			[]string{"type", "status"},
		),
		manualInterventions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mail_service_manual_interventions_total",
				Help: "Manual intervention requests",
			},
			[]string{"reason"},
		),
		sessionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mail_service_sessions_active",
			Help: "Number of active registration sessions",
		}),
		sessionsDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "mail_service_sessions_duration_seconds",
			Help:    "Duration of registration sessions",
			Buckets: prometheus.ExponentialBuckets(30, 2, 10),
		}),
	}
}

// IncrementRegistrationAttempts increments registration attempts counter
func (m *MetricsCollector) IncrementRegistrationAttempts() {
	m.registrationAttempts.Inc()
}

// IncrementRegistrationSuccess increments successful registrations
func (m *MetricsCollector) IncrementRegistrationSuccess() {
	m.registrationSuccess.Inc()
}

// IncrementRegistrationFailures increments failed registrations by reason
func (m *MetricsCollector) IncrementRegistrationFailures(reason string) {
	m.registrationFailures.WithLabelValues(reason).Inc()
}

// RecordRegistrationDuration records registration duration
func (m *MetricsCollector) RecordRegistrationDuration(duration time.Duration) {
	m.registrationDuration.Observe(duration.Seconds())
}

// RecordStepDuration records duration for a specific step
func (m *MetricsCollector) RecordStepDuration(step string, duration time.Duration) {
	m.stepDuration.WithLabelValues(step).Observe(duration.Seconds())
}

// IncrementProxyUsage increments proxy usage metric
func (m *MetricsCollector) IncrementProxyUsage(proxyType, status string) {
	m.proxyUsage.WithLabelValues(proxyType, status).Inc()
}

// IncrementSMSVerification increments SMS verification metric
func (m *MetricsCollector) IncrementSMSVerification(status string) {
	m.smsVerifications.WithLabelValues(status).Inc()
}

// IncrementCaptchaSolved increments captcha solving metric
func (m *MetricsCollector) IncrementCaptchaSolved(captchaType, status string) {
	m.captchaSolved.WithLabelValues(captchaType, status).Inc()
}

// IncrementManualIntervention increments manual intervention requests
func (m *MetricsCollector) IncrementManualIntervention(reason string) {
	m.manualInterventions.WithLabelValues(reason).Inc()
}

// SetActiveSessions sets the number of active sessions
func (m *MetricsCollector) SetActiveSessions(count float64) {
	m.sessionsActive.Set(count)
}

// RecordSessionDuration records session duration
func (m *MetricsCollector) RecordSessionDuration(duration time.Duration) {
	m.sessionsDuration.Observe(duration.Seconds())
}
