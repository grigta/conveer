package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	tasksTotal       *prometheus.CounterVec
	tasksActive      *prometheus.GaugeVec
	actionsTotal     *prometheus.CounterVec
	actionDuration   *prometheus.HistogramVec
	taskDuration     *prometheus.HistogramVec
	errorsTotal      *prometheus.CounterVec
	accountsReady    *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		tasksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "warming_tasks_total",
				Help: "Total number of warming tasks created",
			},
			[]string{"platform", "scenario_type", "status"},
		),

		tasksActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "warming_tasks_active",
				Help: "Number of currently active warming tasks",
			},
			[]string{"platform"},
		),

		actionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "warming_actions_total",
				Help: "Total number of warming actions executed",
			},
			[]string{"platform", "action_type", "status"},
		),

		actionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "warming_action_duration_seconds",
				Help:    "Duration of warming action execution in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
			},
			[]string{"platform", "action_type"},
		),

		taskDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "warming_task_duration_days",
				Help:    "Duration of warming task completion in days",
				Buckets: []float64{1, 3, 7, 14, 21, 30, 45, 60},
			},
			[]string{"platform", "scenario_type"},
		),

		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "warming_errors_total",
				Help: "Total number of warming errors",
			},
			[]string{"platform", "error_type"},
		),

		accountsReady: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "warming_accounts_ready_total",
				Help: "Total number of accounts that completed warming",
			},
			[]string{"platform"},
		),
	}
}

func (m *Metrics) Register() {
	// Metrics are auto-registered by promauto
}

func (m *Metrics) IncrementTasksTotal(platform, scenarioType, status string) {
	m.tasksTotal.WithLabelValues(platform, scenarioType, status).Inc()
}

func (m *Metrics) SetTasksActive(platform string, count float64) {
	m.tasksActive.WithLabelValues(platform).Set(count)
}

func (m *Metrics) IncrementTasksActive(platform string) {
	m.tasksActive.WithLabelValues(platform).Inc()
}

func (m *Metrics) DecrementTasksActive(platform string) {
	m.tasksActive.WithLabelValues(platform).Dec()
}

func (m *Metrics) IncrementActionsTotal(platform, actionType, status string) {
	m.actionsTotal.WithLabelValues(platform, actionType, status).Inc()
}

func (m *Metrics) ObserveActionDuration(platform, actionType string, seconds float64) {
	m.actionDuration.WithLabelValues(platform, actionType).Observe(seconds)
}

func (m *Metrics) ObserveTaskDuration(platform, scenarioType string, days float64) {
	m.taskDuration.WithLabelValues(platform, scenarioType).Observe(days)
}

func (m *Metrics) IncrementErrorsTotal(platform, errorType string) {
	m.errorsTotal.WithLabelValues(platform, errorType).Inc()
}

func (m *Metrics) IncrementAccountsReady(platform string) {
	m.accountsReady.WithLabelValues(platform).Inc()
}
