package database

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakePoolStatsSource struct {
	stats PoolStats
}

func (source *fakePoolStatsSource) PoolStats() PoolStats {
	return source.stats
}

func TestPoolCollector(t *testing.T) {
	source := &fakePoolStatsSource{stats: PoolStats{
		AcquiredConns:           2,
		IdleConns:               3,
		TotalConns:              6,
		ConstructingConns:       1,
		MaxConns:                10,
		AcquireCount:            100,
		CanceledAcquireCount:    4,
		EmptyAcquireCount:       20,
		NewConnsCount:           8,
		MaxLifetimeDestroyCount: 2,
		MaxIdleDestroyCount:     3,
		AcquireDuration:         1500 * time.Millisecond,
		EmptyAcquireWaitTime:    250 * time.Millisecond,
	}}
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewPoolCollector(source))

	want := `
# HELP store_db_pool_acquire_duration_seconds_total Cumulative duration of successful PostgreSQL connection acquisitions.
# TYPE store_db_pool_acquire_duration_seconds_total counter
store_db_pool_acquire_duration_seconds_total 1.5
# HELP store_db_pool_acquired_connections Current number of acquired PostgreSQL connections.
# TYPE store_db_pool_acquired_connections gauge
store_db_pool_acquired_connections 2
# HELP store_db_pool_canceled_acquires_total Total number of canceled PostgreSQL connection acquisitions.
# TYPE store_db_pool_canceled_acquires_total counter
store_db_pool_canceled_acquires_total 4
# HELP store_db_pool_empty_acquire_wait_seconds_total Cumulative wait duration for acquisitions from an empty pool.
# TYPE store_db_pool_empty_acquire_wait_seconds_total counter
store_db_pool_empty_acquire_wait_seconds_total 0.25
# HELP store_db_pool_idle_connections Current number of idle PostgreSQL connections.
# TYPE store_db_pool_idle_connections gauge
store_db_pool_idle_connections 3
# HELP store_db_pool_max_connections Maximum number of PostgreSQL connections allowed.
# TYPE store_db_pool_max_connections gauge
store_db_pool_max_connections 10
# HELP store_db_pool_total_connections Current total number of PostgreSQL connections.
# TYPE store_db_pool_total_connections gauge
store_db_pool_total_connections 6
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(want),
		"store_db_pool_acquire_duration_seconds_total",
		"store_db_pool_acquired_connections",
		"store_db_pool_canceled_acquires_total",
		"store_db_pool_empty_acquire_wait_seconds_total",
		"store_db_pool_idle_connections",
		"store_db_pool_max_connections",
		"store_db_pool_total_connections",
	); err != nil {
		t.Fatal(err)
	}

	source.stats.AcquiredConns = 5
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather updated metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == "store_db_pool_acquired_connections" {
			if got := family.Metric[0].GetGauge().GetValue(); got != 5 {
				t.Fatalf("expected collector to read updated snapshot value 5, got %v", got)
			}
			return
		}
	}
	t.Fatal("updated acquired connections metric not found")
}
