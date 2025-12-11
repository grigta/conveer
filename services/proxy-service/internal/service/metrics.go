package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	proxyAllocationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_allocations_total",
			Help: "Total number of proxy allocations",
		},
		[]string{"type", "country"},
	)

	proxyRotationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_rotations_total",
			Help: "Total number of proxy rotations",
		},
		[]string{"reason"},
	)

	proxyHealthChecksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "proxy_health_checks_total",
			Help: "Total number of proxy health checks",
		},
		[]string{"status"},
	)

	proxyHealthCheckDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "proxy_health_check_duration_seconds",
			Help:    "Duration of proxy health checks in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
		[]string{"proxy_id"},
	)

	proxyFraudScore = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "proxy_fraud_score",
			Help:    "Distribution of proxy fraud scores",
			Buckets: prometheus.LinearBuckets(0, 10, 11),
		},
	)

	proxyLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "proxy_latency_milliseconds",
			Help:    "Proxy latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 10),
		},
	)

	activeProxiesCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_proxies_count",
			Help: "Current number of active proxies",
		},
	)

	proxyBindingsCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "proxy_bindings_count",
			Help: "Current number of proxy bindings",
		},
	)

	proxyReleasesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "proxy_releases_total",
			Help: "Total number of proxy releases",
		},
	)

	proxyAllocationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "proxy_allocation_errors_total",
			Help: "Total number of proxy allocation errors",
		},
	)

	proxyRotationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "proxy_rotation_errors_total",
			Help: "Total number of proxy rotation errors",
		},
	)
)

func RecordProxyAllocation(proxyType, country string) {
	proxyAllocationsTotal.WithLabelValues(proxyType, country).Inc()
}

func RecordProxyRotation(reason string) {
	proxyRotationsTotal.WithLabelValues(reason).Inc()
}

func RecordHealthCheck(status string) {
	proxyHealthChecksTotal.WithLabelValues(status).Inc()
}

func RecordHealthCheckDuration(proxyID string, duration float64) {
	proxyHealthCheckDuration.WithLabelValues(proxyID).Observe(duration)
}

func RecordFraudScore(score float64) {
	proxyFraudScore.Observe(score)
}

func RecordLatency(latency float64) {
	proxyLatency.Observe(latency)
}

func SetActiveProxies(count float64) {
	activeProxiesCount.Set(count)
}

func SetProxyBindings(count float64) {
	proxyBindingsCount.Set(count)
}

func RecordProxyRelease() {
	proxyReleasesTotal.Inc()
}

func RecordAllocationError() {
	proxyAllocationErrors.Inc()
}

func RecordRotationError() {
	proxyRotationErrors.Inc()
}
