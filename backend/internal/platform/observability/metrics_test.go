package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/observability/metricstest"
)

func TestRegistryServesProcessMetrics(t *testing.T) {
	registry := observability.NewRegistry()
	w := httptest.NewRecorder()
	observability.MetricsHandler(registry).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	require.Contains(t, w.Body.String(), "go_goroutines")
	require.Positive(t, metricstest.Value(t, registry, "go_goroutines"))
}

// Два реестра не делят счётчики: так тесты, поднимающие несколько экземпляров API, не мешают друг другу.
func TestRegistriesAreIndependent(t *testing.T) {
	newCounter := func() prometheus.Counter {
		return prometheus.NewCounter(prometheus.CounterOpts{Namespace: observability.MetricsNamespace, Name: "test_total"})
	}
	first, second := observability.NewRegistry(), observability.NewRegistry()
	a := observability.Register(first, newCounter())
	observability.Register(second, newCounter())
	a.Inc()

	require.EqualValues(t, 1, metricstest.Value(t, first, "goden_test_total"))
	require.Zero(t, metricstest.Value(t, second, "goden_test_total"))
}

func TestRegisterReturnsTheExistingMetric(t *testing.T) {
	registry := observability.NewRegistry()
	newCounter := func() *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: observability.MetricsNamespace, Name: "test_total"}, []string{"kind"})
	}
	first := observability.Register(registry, newCounter())
	second := observability.Register(registry, newCounter())
	first.WithLabelValues("a").Inc()
	second.WithLabelValues("a").Inc()

	require.Same(t, first, second)
	require.EqualValues(t, 2, metricstest.Value(t, registry, "goden_test_total", "kind", "a"))
}
