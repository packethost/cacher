package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheDuration prometheus.ObserverVec
	cacheErrors   *prometheus.CounterVec
	cacheHits     *prometheus.CounterVec
	cacheInFlight *prometheus.GaugeVec
	cacheStalls   *prometheus.CounterVec
	cacheTotals   *prometheus.CounterVec

	ingestCount    *prometheus.CounterVec
	ingestErrors   *prometheus.CounterVec
	ingestDuration *prometheus.GaugeVec
)

func setupMetrics(facility string) {
	curryLabels := prometheus.Labels{
		"service":  "cacher",
		"facility": facility,
	}

	cacheDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_ops_duration_seconds",
		Help:    "Duration of cache operations",
		Buckets: prometheus.LinearBuckets(.01, .05, 10),
	}, []string{"service", "facility", "method", "op"}).MustCurryWith(curryLabels)
	cacheErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_ops_errors_total",
		Help: "Number of cache errors.",
	}, []string{"service", "facility", "method", "op"}).MustCurryWith(curryLabels)
	cacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hit_total",
		Help: "Number of cache hits.",
	}, []string{"service", "facility", "method", "op"}).MustCurryWith(curryLabels)
	cacheInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cache_ops_current_total",
		Help: "Number of in flight cache requests.",
	}, []string{"service", "facility", "method", "op"}).MustCurryWith(curryLabels)
	cacheStalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_stall_total",
		Help: "Number of cache stalled due to DB.",
	}, []string{"service", "facility", "method", "op"}).MustCurryWith(curryLabels)
	cacheTotals = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_ops_total",
		Help: "Number of cache ops.",
	}, []string{"service", "facility", "method", "op"}).MustCurryWith(curryLabels)

	sugar.Info("initializing label values")
	var labels []prometheus.Labels

	// Method Push
	labels = []prometheus.Labels{
		{"method": "Push", "op": ""},
	}
	initCounterLabels(cacheErrors, labels)
	initGaugeLabels(cacheInFlight, labels)
	initCounterLabels(cacheStalls, labels)
	initCounterLabels(cacheTotals, labels)
	labels = []prometheus.Labels{
		{"method": "Push", "op": "insert"},
		{"method": "Push", "op": "delete"},
	}
	initObserverLabels(cacheDuration, labels)
	initCounterLabels(cacheHits, labels)

	labels = []prometheus.Labels{
		{"method": "ByMAC", "op": "get"},
		{"method": "ByIP", "op": "get"},
		{"method": "All", "op": "get"},
	}
	initCounterLabels(cacheErrors, labels)
	initGaugeLabels(cacheInFlight, labels)
	initCounterLabels(cacheStalls, labels)
	initCounterLabels(cacheTotals, labels)
	initObserverLabels(cacheDuration, labels)
	initCounterLabels(cacheHits, labels)

	ingestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_op_count_total",
		Help: "Number of attempts made to ingest facility data.",
	}, []string{"service", "facility", "op"}).MustCurryWith(curryLabels)
	ingestDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ingest_op_duration_seconds",
		Help: "Duration of successful ingestion actions while attempting to ingest facility data.",
	}, []string{"service", "facility", "op"}).MustCurryWith(curryLabels)
	ingestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_error_count_total",
		Help: "Number of errors occurred attempting to ingest facility data.",
	}, []string{"service", "facility", "op"}).MustCurryWith(curryLabels)
	labels = []prometheus.Labels{{"op": "fetch"}, {"op": "copy"}}
	initCounterLabels(ingestCount, labels)
	initGaugeLabels(ingestDuration, labels)
	initCounterLabels(ingestErrors, labels)
}

func initObserverLabels(m prometheus.ObserverVec, l []prometheus.Labels) {
	for _, labels := range l {
		m.With(labels)
	}
}

func initGaugeLabels(m *prometheus.GaugeVec, l []prometheus.Labels) {
	for _, labels := range l {
		m.With(labels)
	}
}

func initCounterLabels(m *prometheus.CounterVec, l []prometheus.Labels) {
	for _, labels := range l {
		m.With(labels)
	}
}
