package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricsCollector struct {
	purchaseSuccess   *prometheus.CounterVec
	purchaseFailed    *prometheus.CounterVec
	purchasePrice     *prometheus.HistogramVec
	codeReceived      *prometheus.CounterVec
	cancellations     *prometheus.CounterVec
	activationDuration *prometheus.HistogramVec
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		purchaseSuccess: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sms_purchase_success_total",
				Help: "Total number of successful phone number purchases",
			},
			[]string{"provider", "service"},
		),
		purchaseFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sms_purchase_failed_total",
				Help: "Total number of failed phone number purchases",
			},
			[]string{"provider", "service"},
		),
		purchasePrice: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sms_purchase_price",
				Help:    "Price distribution of phone number purchases",
				Buckets: prometheus.LinearBuckets(0, 50, 20),
			},
			[]string{"provider"},
		),
		codeReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sms_code_received_total",
				Help: "Total number of SMS codes received",
			},
			[]string{"provider", "service"},
		),
		cancellations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sms_cancellations_total",
				Help: "Total number of activation cancellations",
			},
			[]string{"provider", "service", "refunded"},
		),
		activationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sms_activation_duration_seconds",
				Help:    "Duration of activations from purchase to code receipt",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10),
			},
			[]string{"provider", "service"},
		),
	}
}

func (m *MetricsCollector) IncrementPurchaseSuccess(provider, service string) {
	m.purchaseSuccess.WithLabelValues(provider, service).Inc()
}

func (m *MetricsCollector) IncrementPurchaseFailed(provider, service string) {
	m.purchaseFailed.WithLabelValues(provider, service).Inc()
}

func (m *MetricsCollector) RecordPurchasePrice(provider string, price float64) {
	m.purchasePrice.WithLabelValues(provider).Observe(price)
}

func (m *MetricsCollector) IncrementCodeReceived(provider, service string) {
	m.codeReceived.WithLabelValues(provider, service).Inc()
}

func (m *MetricsCollector) IncrementCancellation(provider, service string, refunded bool) {
	refundedStr := "false"
	if refunded {
		refundedStr = "true"
	}
	m.cancellations.WithLabelValues(provider, service, refundedStr).Inc()
}

func (m *MetricsCollector) RecordActivationDuration(provider, service string, duration float64) {
	m.activationDuration.WithLabelValues(provider, service).Observe(duration)
}
