package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics stores Prometheus collectors used across the service.
type Metrics struct {
	WAIncomingMessages *prometheus.CounterVec
	WAOutgoingMessages *prometheus.CounterVec
	GeminiRequests     *prometheus.CounterVec
	GeminiLatency      *prometheus.HistogramVec
	AtlanticRequests   *prometheus.CounterVec
	AtlanticLatency    *prometheus.HistogramVec
	Errors             *prometheus.CounterVec
}

var (
	regOnce         sync.Once
	metricsInstance *Metrics
)

// Registry builds and registers the metrics singleton with optional namespace.
func Registry(namespace string) *Metrics {
	regOnce.Do(func() {
		metricsInstance = &Metrics{
			WAIncomingMessages: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "wa_incoming_messages_total",
				Help:      "Total incoming WhatsApp messages processed.",
			}, []string{"type"}),
			WAOutgoingMessages: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "wa_outgoing_messages_total",
				Help:      "Total outgoing WhatsApp messages sent.",
			}, []string{"type"}),
			GeminiRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "gemini_requests_total",
				Help:      "Total Gemini API requests by outcome.",
			}, []string{"status"}),
			GeminiLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "gemini_request_duration_seconds",
				Help:      "Latency distribution for Gemini API calls.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"status"}),
			AtlanticRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "atlantic_requests_total",
				Help:      "Total Atlantic API requests by endpoint and status.",
			}, []string{"endpoint", "status"}),
			AtlanticLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "atlantic_request_duration_seconds",
				Help:      "Latency distribution for Atlantic API requests.",
				Buckets:   prometheus.DefBuckets,
			}, []string{"endpoint", "status"}),
			Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total errors grouped by component.",
			}, []string{"component"}),
		}

		prometheus.MustRegister(
			metricsInstance.WAIncomingMessages,
			metricsInstance.WAOutgoingMessages,
			metricsInstance.GeminiRequests,
			metricsInstance.GeminiLatency,
			metricsInstance.AtlanticRequests,
			metricsInstance.AtlanticLatency,
			metricsInstance.Errors,
		)
	})
	return metricsInstance
}
