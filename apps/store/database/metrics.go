package database

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PoolStats struct {
	AcquiredConns           int32
	IdleConns               int32
	TotalConns              int32
	ConstructingConns       int32
	MaxConns                int32
	AcquireCount            int64
	CanceledAcquireCount    int64
	EmptyAcquireCount       int64
	NewConnsCount           int64
	MaxLifetimeDestroyCount int64
	MaxIdleDestroyCount     int64
	AcquireDuration         time.Duration
	EmptyAcquireWaitTime    time.Duration
}

type PoolStatsSource interface {
	PoolStats() PoolStats
}

func (db *DB) PoolStats() PoolStats {
	stats := db.pool.Stat()
	return PoolStats{
		AcquiredConns:           stats.AcquiredConns(),
		IdleConns:               stats.IdleConns(),
		TotalConns:              stats.TotalConns(),
		ConstructingConns:       stats.ConstructingConns(),
		MaxConns:                stats.MaxConns(),
		AcquireCount:            stats.AcquireCount(),
		CanceledAcquireCount:    stats.CanceledAcquireCount(),
		EmptyAcquireCount:       stats.EmptyAcquireCount(),
		NewConnsCount:           stats.NewConnsCount(),
		MaxLifetimeDestroyCount: stats.MaxLifetimeDestroyCount(),
		MaxIdleDestroyCount:     stats.MaxIdleDestroyCount(),
		AcquireDuration:         stats.AcquireDuration(),
		EmptyAcquireWaitTime:    stats.EmptyAcquireWaitTime(),
	}
}

type poolCollector struct {
	source      PoolStatsSource
	descriptors map[string]*prometheus.Desc
}

func NewPoolCollector(source PoolStatsSource) prometheus.Collector {
	descriptions := map[string]string{
		"acquired_connections":                     "Current number of acquired PostgreSQL connections.",
		"idle_connections":                         "Current number of idle PostgreSQL connections.",
		"total_connections":                        "Current total number of PostgreSQL connections.",
		"constructing_connections":                 "Current number of PostgreSQL connections being constructed.",
		"max_connections":                          "Maximum number of PostgreSQL connections allowed.",
		"acquires_total":                           "Total number of successful PostgreSQL connection acquisitions.",
		"canceled_acquires_total":                  "Total number of canceled PostgreSQL connection acquisitions.",
		"empty_acquires_total":                     "Total successful acquisitions that waited for an empty pool.",
		"new_connections_total":                    "Total number of new PostgreSQL connections.",
		"max_lifetime_destroyed_connections_total": "Total connections destroyed due to maximum lifetime.",
		"max_idle_destroyed_connections_total":     "Total connections destroyed due to maximum idle time.",
		"acquire_duration_seconds_total":           "Cumulative duration of successful PostgreSQL connection acquisitions.",
		"empty_acquire_wait_seconds_total":         "Cumulative wait duration for acquisitions from an empty pool.",
	}
	descriptors := make(map[string]*prometheus.Desc, len(descriptions))
	for name, help := range descriptions {
		descriptors[name] = prometheus.NewDesc("store_db_pool_"+name, help, nil, nil)
	}
	return &poolCollector{source: source, descriptors: descriptors}
}

func (collector *poolCollector) Describe(descriptions chan<- *prometheus.Desc) {
	for _, description := range collector.descriptors {
		descriptions <- description
	}
}

func (collector *poolCollector) Collect(metrics chan<- prometheus.Metric) {
	stats := collector.source.PoolStats()
	collector.send(metrics, "acquired_connections", prometheus.GaugeValue, float64(stats.AcquiredConns))
	collector.send(metrics, "idle_connections", prometheus.GaugeValue, float64(stats.IdleConns))
	collector.send(metrics, "total_connections", prometheus.GaugeValue, float64(stats.TotalConns))
	collector.send(metrics, "constructing_connections", prometheus.GaugeValue, float64(stats.ConstructingConns))
	collector.send(metrics, "max_connections", prometheus.GaugeValue, float64(stats.MaxConns))
	collector.send(metrics, "acquires_total", prometheus.CounterValue, float64(stats.AcquireCount))
	collector.send(metrics, "canceled_acquires_total", prometheus.CounterValue, float64(stats.CanceledAcquireCount))
	collector.send(metrics, "empty_acquires_total", prometheus.CounterValue, float64(stats.EmptyAcquireCount))
	collector.send(metrics, "new_connections_total", prometheus.CounterValue, float64(stats.NewConnsCount))
	collector.send(metrics, "max_lifetime_destroyed_connections_total", prometheus.CounterValue, float64(stats.MaxLifetimeDestroyCount))
	collector.send(metrics, "max_idle_destroyed_connections_total", prometheus.CounterValue, float64(stats.MaxIdleDestroyCount))
	collector.send(metrics, "acquire_duration_seconds_total", prometheus.CounterValue, stats.AcquireDuration.Seconds())
	collector.send(metrics, "empty_acquire_wait_seconds_total", prometheus.CounterValue, stats.EmptyAcquireWaitTime.Seconds())
}

func (collector *poolCollector) send(metrics chan<- prometheus.Metric, name string, valueType prometheus.ValueType, value float64) {
	metrics <- prometheus.MustNewConstMetric(collector.descriptors[name], valueType, value)
}
