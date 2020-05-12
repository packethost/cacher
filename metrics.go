package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheCountTotal prometheus.Gauge
	cacheDuration   prometheus.ObserverVec
	cacheErrors     *prometheus.CounterVec
	cacheHits       *prometheus.CounterVec
	cacheInFlight   *prometheus.GaugeVec
	cacheStalls     *prometheus.CounterVec
	cacheTotals     *prometheus.CounterVec

	cacherState prometheus.Gauge

	ingestCount    *prometheus.CounterVec
	ingestDuration *prometheus.GaugeVec
	ingestErrors   *prometheus.CounterVec

	watchMissTotal prometheus.Counter
)

func setupMetrics(facility string) {
	cacheCountTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cache_count_total",
		Help: "Number of in devices in memory.",
	})
	cacheDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cache_ops_duration_seconds",
		Help:    "Duration of cache operations",
		Buckets: prometheus.LinearBuckets(.01, .1, 10),
	}, []string{"method", "op"})
	cacheErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_ops_errors_total",
		Help: "Number of cache errors.",
	}, []string{"method", "op"})
	cacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hit_total",
		Help: "Number of cache hits.",
	}, []string{"method", "op"})
	cacheInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cache_ops_current_total",
		Help: "Number of in flight cache requests.",
	}, []string{"method", "op"})
	cacheStalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_stall_total",
		Help: "Number of cache stalled due to DB.",
	}, []string{"method", "op"})
	cacheTotals = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_ops_total",
		Help: "Number of cache ops.",
	}, []string{"method", "op"})

	logger.Info("initializing label values")
	var labels []prometheus.Labels

	labels = []prometheus.Labels{
		{"method": "Push", "op": ""},
		{"method": "Ingest", "op": ""},
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
		{"method": "ByID", "op": "get"},
		{"method": "All", "op": "get"},
		{"method": "Ingest", "op": ""},
		{"method": "Watch", "op": "get"},
		{"method": "Watch", "op": "push"},
	}
	initCounterLabels(cacheErrors, labels)
	initGaugeLabels(cacheInFlight, labels)
	initCounterLabels(cacheStalls, labels)
	initCounterLabels(cacheTotals, labels)
	initObserverLabels(cacheDuration, labels)
	initCounterLabels(cacheHits, labels)

	cacherState = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cacher_state",
		Help: "Reports cacher state, 0:started, 1:ingesting, 2:ready",
	})

	ingestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_op_count_total",
		Help: "Number of attempts made to ingest facility data.",
	}, []string{"method", "op"})
	ingestDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ingest_op_duration_seconds",
		Help: "Duration of successful ingestion actions while attempting to ingest facility data.",
	}, []string{"method", "op"})
	ingestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_error_count_total",
		Help: "Number of errors occurred attempting to ingest facility data.",
	}, []string{"method", "op"})
	labels = []prometheus.Labels{
		{"method": "Ingest", "op": ""},
		{"method": "Ingest", "op": "fetch"},
		{"method": "Ingest", "op": "copy"},
	}
	initCounterLabels(ingestCount, labels)
	initGaugeLabels(ingestDuration, labels)
	initCounterLabels(ingestErrors, labels)

	watchMissTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "watch_miss_count_total",
		Help: "Number of missed updates due to a blocked channel.",
	})
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
