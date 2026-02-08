package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Deployment metrics
	DeploymentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecs_deployments_total",
			Help: "Total number of deployments attempted",
		},
		[]string{"strategy", "status"},
	)

	DeploymentDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ecs_deployment_duration_seconds",
			Help:    "Deployment duration in seconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10),
		},
		[]string{"strategy"},
	)

	DeploymentsInProgress = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ecs_deployments_in_progress",
			Help: "Number of deployments currently in progress",
		},
	)

	// AWS API metrics
	AWSAPICallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecs_aws_api_calls_total",
			Help: "Total number of AWS API calls",
		},
		[]string{"service", "operation", "status"},
	)

	AWSAPICallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ecs_aws_api_call_duration_seconds",
			Help:    "AWS API call duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"service", "operation"},
	)

	// Strategy-specific metrics
	CanaryStagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecs_canary_stages_total",
			Help: "Total canary stages completed",
		},
		[]string{"stage", "status"},
	)

	TrafficShiftsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecs_traffic_shifts_total",
			Help: "Total traffic shifts performed",
		},
		[]string{"strategy", "status"},
	)

	// Error metrics
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecs_errors_total",
			Help: "Total errors encountered",
		},
		[]string{"component", "error_type"},
	)
)

// RecordDeployment records a deployment attempt
func RecordDeployment(strategy, status string, duration time.Duration) {
	DeploymentsTotal.WithLabelValues(strategy, status).Inc()
	DeploymentDuration.WithLabelValues(strategy).Observe(duration.Seconds())
}

// RecordAWSCall records an AWS API call
func RecordAWSCall(service, operation, status string, duration time.Duration) {
	AWSAPICallsTotal.WithLabelValues(service, operation, status).Inc()
	AWSAPICallDuration.WithLabelValues(service, operation).Observe(duration.Seconds())
}

// RecordError records an error
func RecordError(component, errorType string) {
	ErrorsTotal.WithLabelValues(component, errorType).Inc()
}

// IncrementInProgress increments in-progress deployments
func IncrementInProgress() {
	DeploymentsInProgress.Inc()
}

// DecrementInProgress decrements in-progress deployments
func DecrementInProgress() {
	DeploymentsInProgress.Dec()
}
