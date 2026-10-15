// Package httpx содержит общую HTTP-обвязку процессов: middleware, запуск сервера
// с корректным завершением, health-эндпоинты и запись ошибок в формате RFC 9457.
// О доменных модулях и контракте API пакет не знает.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
)

// CorrelationHeader - заголовок, связывающий запрос с логами и событиями.
const CorrelationHeader = "X-Correlation-Id"

// Problem - описание ошибки по RFC 9457. Поля совпадают со схемой Problem в api/openapi.yaml.
type Problem struct {
	Type          string       `json:"type"`
	Title         string       `json:"title"`
	Status        int          `json:"status"`
	Detail        string       `json:"detail,omitempty"`
	CorrelationID *uuid.UUID   `json:"correlation_id,omitempty"`
	Errors        []FieldError `json:"errors,omitempty"`
}

// FieldError - некорректное поле запроса.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	// Code и Limit - вид ошибки и предел, если модуль их различает.
	Code  string `json:"code,omitempty"`
	Limit *int64 `json:"limit,omitempty"`
}

// WriteProblem пишет ошибку с типом содержимого application/problem+json.
// Correlation ID берётся из контекста запроса.
func WriteProblem(ctx context.Context, w http.ResponseWriter, p Problem) {
	if id, ok := observability.CorrelationID(ctx); ok {
		p.CorrelationID = &id
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p) // клиент мог уже отключиться, сделать с этим нечего
}

// Middleware оборачивает обработчик.
type Middleware func(http.Handler) http.Handler

// Chain применяет middleware так, что первое в списке оказывается внешним.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// Correlation кладёт correlation ID в контекст запроса и возвращает его в заголовке ответа.
// Идентификатор клиента принимается, только если это корректный UUID; иначе создаётся новый.
func Correlation() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(r.Header.Get(CorrelationHeader))
			if err != nil {
				id = uuid.Must(uuid.NewV7())
			}
			w.Header().Set(CorrelationHeader, id.String())
			next.ServeHTTP(w, r.WithContext(observability.WithCorrelationID(r.Context(), id)))
		})
	}
}

// TextBody отклоняет запрос, тело которого больше maxBytes или не является корректным UTF-8.
// Разбор JSON кодировку не проверяет, и такой текст дошёл бы до базы, где вызвал бы
// ошибку, неотличимую от внутреннего сбоя.
func TextBody(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
			var tooLarge *http.MaxBytesError
			switch {
			case errors.As(err, &tooLarge):
				WriteProblem(r.Context(), w, Problem{
					Type: "bad-request", Title: "Тело запроса слишком большое", Status: http.StatusRequestEntityTooLarge,
				})
				return
			case err != nil:
				WriteProblem(r.Context(), w, Problem{
					Type: "bad-request", Title: "Не удалось прочитать тело запроса", Status: http.StatusBadRequest,
				})
				return
			case !utf8.Valid(body):
				WriteProblem(r.Context(), w, Problem{
					Type: "bad-request", Title: "Тело запроса содержит некорректный UTF-8", Status: http.StatusBadRequest,
				})
				return
			// Нулевой символ проходит проверку UTF-8 и разбор JSON, но PostgreSQL его не хранит:
			// запрос завершался бы ошибкой 500.
			// упрощение: проверка по тексту тела отклонит и строку с экранированной обратной косой
			// чертой перед u0000; точная проверка требует разбора JSON
			case bytes.IndexByte(body, 0) >= 0 || bytes.Contains(body, []byte(`\u0000`)):
				WriteProblem(r.Context(), w, Problem{
					Type: "bad-request", Title: "Тело запроса содержит нулевой символ", Status: http.StatusBadRequest,
				})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

// Recover превращает панику обработчика в ответ 500 и запись в лог.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered) // так net/http просит оборвать соединение без записи в лог
				}
				logger.ErrorContext(r.Context(), "panic in http handler", "panic", fmt.Sprint(recovered), "path", r.URL.Path)
				WriteProblem(r.Context(), w, Problem{
					Type: "internal-error", Title: "Внутренняя ошибка", Status: http.StatusInternalServerError,
				})
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog пишет одну запись на запрос: метод, путь, статус, длительность.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			level := slog.LevelInfo
			if recorder.status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			logger.Log(r.Context(), level, "http request",
				"method", r.Method, "path", r.URL.Path, "status", recorder.status,
				"duration_ms", time.Since(started).Milliseconds())
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap даёт http.ResponseController доступ к исходному ResponseWriter.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

type actorKey struct{}

// WithActor помещает автора запроса в контекст.
func WithActor(ctx context.Context, actor eventstore.Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

// Actor возвращает автора запроса: от его имени выполняются команды.
func Actor(ctx context.Context) (eventstore.Actor, bool) {
	actor, ok := ctx.Value(actorKey{}).(eventstore.Actor)
	return actor, ok
}

// CORS разрешает кросс-доменные запросы браузера с перечисленных адресов (схема, хост, порт).
// Предварительный запрос OPTIONS получает ответ сразу, без токена. Запрос с другого адреса
// разрешающих заголовков не получает, и браузер не отдаёт ответ странице.
func CORS(origins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Add("Vary", "Origin")
			if origin == "" || !slices.Contains(origins, origin) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", CorrelationHeader)
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, Last-Event-ID, "+CorrelationHeader)
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Health возвращает обработчик с /healthz и /readyz.
// ready проверяет готовность процесса, обычно доступность базы данных.
func Health(ready func(ctx context.Context) error) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// Serve обслуживает запросы на addr до отмены ctx, затем перестаёт принимать соединения
// и ждёт завершения текущих запросов не дольше shutdownTimeout.
func Serve(ctx context.Context, addr string, handler http.Handler, shutdownTimeout time.Duration, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("httpx: listen %s: %w", addr, err)
	}
	return ServeListener(ctx, listener, handler, shutdownTimeout, logger)
}

// ServeListener делает то же, что Serve, на готовом слушателе.
func ServeListener(
	ctx context.Context, listener net.Listener, handler http.Handler, shutdownTimeout time.Duration, logger *slog.Logger,
) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	logger.InfoContext(ctx, "http server started", "addr", listener.Addr().String())

	select {
	case err := <-failed:
		return fmt.Errorf("httpx: serve: %w", err)
	case <-ctx.Done():
	}

	// контекст запуска уже отменён, для завершения нужен свой
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("httpx: shutdown: %w", err)
	}
	logger.InfoContext(shutdownCtx, "http server stopped")
	return nil
}
