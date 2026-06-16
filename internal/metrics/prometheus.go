package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus collectors for VelocityPay.
type Metrics struct {
	// HTTP
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Business
	UsersTotal        prometheus.Counter
	WalletsTotal      prometheus.Counter
	TransactionsTotal *prometheus.CounterVec
	TransactionAmount *prometheus.HistogramVec

	// Infrastructure
	RedisCacheHits   prometheus.Counter
	RedisCacheMisses prometheus.Counter
	RabbitMQPublished *prometheus.CounterVec
}

// New registers and returns all application metrics.
func New() *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "velocitypay",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),

		UsersTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "users_registered_total",
			Help:      "Total number of registered users.",
		}),

		WalletsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "wallets_created_total",
			Help:      "Total number of wallets created.",
		}),

		TransactionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "transactions_total",
			Help:      "Total transactions by status.",
		}, []string{"status"}),

		TransactionAmount: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "velocitypay",
			Name:      "transaction_amount",
			Help:      "Distribution of transaction amounts.",
			Buckets:   []float64{10, 50, 100, 500, 1000, 5000, 10000},
		}, []string{"type"}),

		RedisCacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "redis_cache_hits_total",
			Help:      "Total Redis cache hits.",
		}),

		RedisCacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "redis_cache_misses_total",
			Help:      "Total Redis cache misses.",
		}),

		RabbitMQPublished: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "velocitypay",
			Name:      "rabbitmq_published_total",
			Help:      "Total events published to RabbitMQ.",
		}, []string{"routing_key"}),
	}
}
