package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/platform/observability"
)

// unknownOperation - метка запросов, не попавших ни в одну операцию контракта.
const unknownOperation = "unknown"

// requestMetrics считает запросы API и время ответа. Метка - имя операции контракта,
// а не путь: идентификаторы из пути в метки не попадают, и число серий ограничено контрактом.
func requestMetrics(registry prometheus.Registerer, server *oas.Server) httpx.Middleware {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: observability.MetricsNamespace, Name: "http_requests_total",
		Help: "API requests by contract operation and response status.",
	}, []string{"operation", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: observability.MetricsNamespace, Name: "http_request_duration_seconds",
		Help:    "API response time by contract operation.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"operation"})
	registry.MustRegister(requests, duration)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			operation := unknownOperation
			if route, found := server.FindPath(r.Method, r.URL); found {
				operation = route.Name()
			}
			started := time.Now() //nolint:forbidigo // длительность запроса, а не время предметной области
			recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			requests.WithLabelValues(operation, strconv.Itoa(recorder.status)).Inc()
			duration.WithLabelValues(operation).Observe(time.Since(started).Seconds())
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap даёт http.ResponseController доступ к исходному ResponseWriter.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
