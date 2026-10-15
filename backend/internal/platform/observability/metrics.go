package observability

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsNamespace - префикс имён всех метрик проекта.
const MetricsNamespace = "goden"

// NewRegistry создаёт реестр метрик процесса со стандартными метриками Go и процесса.
// Реестр свой, а не глобальный: несколько экземпляров API в одном процессе тестов
// не делят счётчики и не спорят за имена.
func NewRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registry
}

// RegisterPool добавляет состояние пула соединений PostgreSQL. Значения читаются при каждом опросе.
func RegisterPool(registry prometheus.Registerer, pool *pgxpool.Pool) {
	states := map[string]func() float64{
		"max":      func() float64 { return float64(pool.Stat().MaxConns()) },
		"acquired": func() float64 { return float64(pool.Stat().AcquiredConns()) },
		"idle":     func() float64 { return float64(pool.Stat().IdleConns()) },
	}
	for state, value := range states {
		registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: MetricsNamespace, Name: "db_pool_connections",
			Help:        "Connections of the PostgreSQL pool by state.",
			ConstLabels: prometheus.Labels{"state": state},
		}, value))
	}
	registry.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Namespace: MetricsNamespace, Name: "db_pool_waits_total",
		Help: "Acquires that had to wait because every connection of the pool was busy.",
	}, func() float64 { return float64(pool.Stat().EmptyAcquireCount()) }))
}

// MetricsHandler отдаёт метрики реестра в формате Prometheus.
func MetricsHandler(registry *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry})
}

// Register добавляет метрику в реестр и возвращает её. Если метрика с таким описанием
// уже есть (второй сервис на том же реестре), возвращается зарегистрированная ранее.
func Register[T prometheus.Collector](registry prometheus.Registerer, collector T) T {
	if err := registry.Register(collector); err != nil {
		var registered prometheus.AlreadyRegisteredError
		if errors.As(err, &registered) {
			return registered.ExistingCollector.(T)
		}
		panic(err)
	}
	return collector
}
