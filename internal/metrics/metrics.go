package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// RecordedTotal counts events persisted by the worker by status
	// (success|error).
	RecordedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_recorded_total",
		Help: "Number of activity events handled by the worker",
	}, []string{"status"})

	// Duration observes the time taken to persist a single event.
	Duration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "events_record_duration_seconds",
		Help:    "Activity event persistence duration in seconds",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(RecordedTotal, Duration)
}

func RecordedSuccess() { RecordedTotal.WithLabelValues("success").Inc() }

func RecordedError() { RecordedTotal.WithLabelValues("error").Inc() }