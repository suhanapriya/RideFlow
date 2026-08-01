package database

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// PoolHealthMetrics exposes detailed pool health metrics
type PoolHealthMetrics struct {
    acquireCount    prometheus.Counter
    acquireDuration prometheus.Histogram
    acquiredConns   prometheus.Gauge
    idleConns       prometheus.Gauge
    totalConns      prometheus.Gauge
    maxConns        prometheus.Gauge
    constructing    prometheus.Gauge
    canceledAcquire prometheus.Counter
    emptyAcquire    prometheus.Counter
}

// NewPoolHealthMetrics creates detailed pool health metrics for a service
func NewPoolHealthMetrics(serviceName string) *PoolHealthMetrics {
    return &PoolHealthMetrics{
        acquireCount: promauto.NewCounter(prometheus.CounterOpts{
            Name: serviceName + "_db_pool_acquire_count_total",
            Help: "Total number of successful connection acquisitions from the pool",
        }),
        acquireDuration: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    serviceName + "_db_pool_acquire_duration_seconds",
            Help:    "Time spent acquiring a connection from the pool",
            Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
        }),
        acquiredConns: promauto.NewGauge(prometheus.GaugeOpts{
            Name: serviceName + "_db_pool_acquired_connections",
            Help: "Number of currently acquired connections",
        }),
        idleConns: promauto.NewGauge(prometheus.GaugeOpts{
            Name: serviceName + "_db_pool_idle_connections",
            Help: "Number of idle connections in the pool",
        }),
        totalConns: promauto.NewGauge(prometheus.GaugeOpts{
            Name: serviceName + "_db_pool_total_connections",
            Help: "Total number of connections in the pool",
        }),
        maxConns: promauto.NewGauge(prometheus.GaugeOpts{
            Name: serviceName + "_db_pool_max_connections",
            Help: "Maximum number of connections allowed in the pool",
        }),
        constructing: promauto.NewGauge(prometheus.GaugeOpts{
            Name: serviceName + "_db_pool_constructing_connections",
            Help: "Number of connections being constructed",
        }),
        canceledAcquire: promauto.NewCounter(prometheus.CounterOpts{
            Name: serviceName + "_db_pool_canceled_acquire_total",
            Help: "Total number of canceled connection acquisitions",
        }),
        emptyAcquire: promauto.NewCounter(prometheus.CounterOpts{
            Name: serviceName + "_db_pool_empty_acquire_total",
            Help: "Total number of acquires when pool was empty",
        }),
    }
}

// StartCollecting begins periodic collection of pool stats
func (m *PoolHealthMetrics) StartCollecting(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
    if interval <= 0 {
        interval = 15 * time.Second
    }

    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                m.collect(pool)
            }
        }
    }()
}

func (m *PoolHealthMetrics) collect(pool *pgxpool.Pool) {
    stats := pool.Stat()

    m.acquiredConns.Set(float64(stats.AcquiredConns()))
    m.idleConns.Set(float64(stats.IdleConns()))
    m.totalConns.Set(float64(stats.TotalConns()))
    m.maxConns.Set(float64(stats.MaxConns()))
    m.constructing.Set(float64(stats.ConstructingConns()))
}

// PoolHealthSnapshot returns a point-in-time snapshot of pool health
type PoolHealthSnapshot struct {
    TotalConns        int32   `json:"total_connections"`
    AcquiredConns     int32   `json:"acquired_connections"`
    IdleConns         int32   `json:"idle_connections"`
    ConstructingConns int32   `json:"constructing_connections"`
    MaxConns          int32   `json:"max_connections"`
    Utilization       float64 `json:"utilization_percent"`
}

// GetPoolHealth returns current pool health snapshot
func GetPoolHealth(pool *pgxpool.Pool) PoolHealthSnapshot {
    stats := pool.Stat()
    maxConns := stats.MaxConns()
    utilization := 0.0
    if maxConns > 0 {
        utilization = float64(stats.AcquiredConns()) / float64(maxConns) * 100
    }

    return PoolHealthSnapshot{
        TotalConns:        stats.TotalConns(),
        AcquiredConns:     stats.AcquiredConns(),
        IdleConns:         stats.IdleConns(),
        ConstructingConns: stats.ConstructingConns(),
        MaxConns:          maxConns,
        Utilization:       utilization,
    }
}
