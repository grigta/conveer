package service

import (
	"conveer/services/telegram-service/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricsCollector interface {
	IncrementAccountCreated(status models.AccountStatus)
	IncrementAccountStatusChange(from, to models.AccountStatus)
	IncrementRegistrationAttempts()
	IncrementRegistrationSuccess()
	IncrementRegistrationFailure(reason string)
	RecordRegistrationDuration(seconds float64)
	IncrementSMSRequests()
	IncrementSMSSuccess()
	IncrementSMSFailure()
	IncrementProxyRequests()
	IncrementProxySuccess()
	IncrementProxyFailure()
	UpdateBrowserPoolSize(size int)
	IncrementBrowserAcquisitions()
	IncrementBrowserReleases()
	IncrementManualInterventions()
	RecordStepDuration(step string, seconds float64)
}

type metricsCollector struct {
	accountsCreated         *prometheus.CounterVec
	accountStatusChanges    *prometheus.CounterVec
	registrationAttempts    prometheus.Counter
	registrationSuccess     prometheus.Counter
	registrationFailures    *prometheus.CounterVec
	registrationDuration    prometheus.Histogram
	smsRequests            prometheus.Counter
	smsSuccess             prometheus.Counter
	smsFailures            prometheus.Counter
	proxyRequests          prometheus.Counter
	proxySuccess           prometheus.Counter
	proxyFailures          prometheus.Counter
	browserPoolSize        prometheus.Gauge
	browserAcquisitions    prometheus.Counter
	browserReleases        prometheus.Counter
	manualInterventions    prometheus.Counter
	stepDuration           *prometheus.HistogramVec
}

func NewMetricsCollector(namespace string) MetricsCollector {
	return &metricsCollector{
		accountsCreated: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "accounts_created_total",
				Help:      "Total number of accounts created by status",
			},
			[]string{"status"},
		),
		accountStatusChanges: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "account_status_changes_total",
				Help:      "Total number of account status changes",
			},
			[]string{"from", "to"},
		),
		registrationAttempts: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "registration_attempts_total",
				Help:      "Total number of registration attempts",
			},
		),
		registrationSuccess: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "registration_success_total",
				Help:      "Total number of successful registrations",
			},
		),
		registrationFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "registration_failures_total",
				Help:      "Total number of failed registrations by reason",
			},
			[]string{"reason"},
		),
		registrationDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "registration_duration_seconds",
				Help:      "Duration of registration process in seconds",
				Buckets:   []float64{30, 60, 120, 180, 300, 600, 900, 1200, 1800},
			},
		),
		smsRequests: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sms_requests_total",
				Help:      "Total number of SMS requests",
			},
		),
		smsSuccess: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sms_success_total",
				Help:      "Total number of successful SMS verifications",
			},
		),
		smsFailures: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sms_failures_total",
				Help:      "Total number of failed SMS verifications",
			},
		),
		proxyRequests: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "proxy_requests_total",
				Help:      "Total number of proxy requests",
			},
		),
		proxySuccess: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "proxy_success_total",
				Help:      "Total number of successful proxy allocations",
			},
		),
		proxyFailures: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "proxy_failures_total",
				Help:      "Total number of failed proxy allocations",
			},
		),
		browserPoolSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "browser_pool_size",
				Help:      "Current size of browser pool",
			},
		),
		browserAcquisitions: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "browser_acquisitions_total",
				Help:      "Total number of browser acquisitions",
			},
		),
		browserReleases: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "browser_releases_total",
				Help:      "Total number of browser releases",
			},
		),
		manualInterventions: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "manual_interventions_total",
				Help:      "Total number of manual interventions required",
			},
		),
		stepDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "registration_step_duration_seconds",
				Help:      "Duration of each registration step in seconds",
				Buckets:   []float64{5, 10, 20, 30, 60, 120, 180, 300},
			},
			[]string{"step"},
		),
	}
}

func (m *metricsCollector) IncrementAccountCreated(status models.AccountStatus) {
	m.accountsCreated.WithLabelValues(string(status)).Inc()
}

func (m *metricsCollector) IncrementAccountStatusChange(from, to models.AccountStatus) {
	m.accountStatusChanges.WithLabelValues(string(from), string(to)).Inc()
}

func (m *metricsCollector) IncrementRegistrationAttempts() {
	m.registrationAttempts.Inc()
}

func (m *metricsCollector) IncrementRegistrationSuccess() {
	m.registrationSuccess.Inc()
}

func (m *metricsCollector) IncrementRegistrationFailure(reason string) {
	m.registrationFailures.WithLabelValues(reason).Inc()
}

func (m *metricsCollector) RecordRegistrationDuration(seconds float64) {
	m.registrationDuration.Observe(seconds)
}

func (m *metricsCollector) IncrementSMSRequests() {
	m.smsRequests.Inc()
}

func (m *metricsCollector) IncrementSMSSuccess() {
	m.smsSuccess.Inc()
}

func (m *metricsCollector) IncrementSMSFailure() {
	m.smsFailures.Inc()
}

func (m *metricsCollector) IncrementProxyRequests() {
	m.proxyRequests.Inc()
}

func (m *metricsCollector) IncrementProxySuccess() {
	m.proxySuccess.Inc()
}

func (m *metricsCollector) IncrementProxyFailure() {
	m.proxyFailures.Inc()
}

func (m *metricsCollector) UpdateBrowserPoolSize(size int) {
	m.browserPoolSize.Set(float64(size))
}

func (m *metricsCollector) IncrementBrowserAcquisitions() {
	m.browserAcquisitions.Inc()
}

func (m *metricsCollector) IncrementBrowserReleases() {
	m.browserReleases.Inc()
}

func (m *metricsCollector) IncrementManualInterventions() {
	m.manualInterventions.Inc()
}

func (m *metricsCollector) RecordStepDuration(step string, seconds float64) {
	m.stepDuration.WithLabelValues(step).Observe(seconds)
}