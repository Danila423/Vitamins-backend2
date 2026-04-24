package metrics

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Collector struct {
	HTTPRequestTotal    *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPInFlight        prometheus.Gauge

	AuthAttempts *prometheus.CounterVec
}

var (
	defaultCollector *Collector
	defaultRegistry  *prometheus.Registry
	once             sync.Once
)

func initDefault() {
	defaultRegistry = prometheus.NewRegistry()
	defaultRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	c := &Collector{
		HTTPRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "vitamins",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total HTTP requests",
			},
			[]string{"method", "route", "status_class", "outcome"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "vitamins",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
			},
			[]string{"method", "route", "status_class", "outcome"},
		),
		HTTPInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "vitamins",
				Subsystem: "http",
				Name:      "requests_in_flight",
				Help:      "Current number of HTTP requests being served",
			},
		),
		AuthAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "vitamins",
				Subsystem: "auth",
				Name:      "attempts_total",
				Help:      "Auth operation attempts by operation and result",
			},
			[]string{"operation", "result"},
		),
	}

	defaultRegistry.MustRegister(
		c.HTTPRequestTotal,
		c.HTTPRequestDuration,
		c.HTTPInFlight,
		c.AuthAttempts,
	)
	defaultCollector = c
}

func get() *Collector {
	once.Do(initDefault)
	return defaultCollector
}

func Registry() *prometheus.Registry {
	once.Do(initDefault)
	return defaultRegistry
}

func ObserveAuth(operation, result string) {
	operation = normalizeAuthOperation(operation)
	result = normalizeResult(result)
	get().AuthAttempts.WithLabelValues(operation, result).Inc()
}

func normalizeAuthOperation(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "login", "register", "refresh":
		return strings.ToLower(strings.TrimSpace(op))
	default:
		return "unknown"
	}
}

func normalizeResult(result string) string {
	if strings.EqualFold(strings.TrimSpace(result), "success") {
		return "success"
	}
	return "failure"
}
