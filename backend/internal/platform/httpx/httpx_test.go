package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/platform/observability"
)

var quiet = slog.New(slog.DiscardHandler)

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type unreadableBody struct{}

func (unreadableBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestCorrelationKeepsClientID(t *testing.T) {
	clientID := uuid.Must(uuid.NewV7())
	var seen uuid.UUID
	h := httpx.Chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, _ = observability.CorrelationID(r.Context())
	}), httpx.Correlation())
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(httpx.CorrelationHeader, clientID.String())

	w := serve(h, r)

	require.Equal(t, clientID, seen)
	require.Equal(t, clientID.String(), w.Header().Get(httpx.CorrelationHeader))
}

func TestCorrelationReplacesInvalidID(t *testing.T) {
	for _, header := range []string{"", "not-a-uuid"} {
		var seen uuid.UUID
		h := httpx.Chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen, _ = observability.CorrelationID(r.Context())
		}), httpx.Correlation())
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(httpx.CorrelationHeader, header)

		w := serve(h, r)

		require.NotEqual(t, uuid.Nil, seen)
		require.Equal(t, seen.String(), w.Header().Get(httpx.CorrelationHeader))
	}
}

func TestRecoverReturnsProblem(t *testing.T) {
	var logs bytes.Buffer
	logger := observability.NewLogger(&logs, slog.LevelInfo)
	h := httpx.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret internal detail")
	}), httpx.Correlation(), httpx.Recover(logger))

	w := serve(h, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
	var problem httpx.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
	require.Equal(t, http.StatusInternalServerError, problem.Status)
	require.Equal(t, "internal-error", problem.Type)
	require.Equal(t, "Внутренняя ошибка", problem.Title)
	require.NotNil(t, problem.CorrelationID)
	require.NotContains(t, w.Body.String(), "secret internal detail", "panic text must not leak to the client")
	require.Contains(t, logs.String(), "secret internal detail")
	require.Contains(t, logs.String(), problem.CorrelationID.String())
}

func TestAccessLog(t *testing.T) {
	var logs bytes.Buffer
	logger := observability.NewLogger(&logs, slog.LevelInfo)
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}), httpx.Correlation(), httpx.AccessLog(logger))

	w := serve(h, httptest.NewRequest(http.MethodPost, "/api/v1/products", nil))

	var record map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &record))
	require.Equal(t, "POST", record["method"])
	require.Equal(t, "/api/v1/products", record["path"])
	require.EqualValues(t, http.StatusCreated, record["status"])
	require.Equal(t, w.Header().Get(httpx.CorrelationHeader), record["correlation_id"])
	require.Equal(t, "INFO", record["level"])
}

func TestTextBody(t *testing.T) {
	var got []byte
	h := httpx.Chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}), httpx.TextBody(16))

	tests := []struct {
		name      string
		body      []byte
		status    int
		wantTitle string
	}{
		{"valid utf-8", []byte(`{"n":"Соль"}`), http.StatusOK, ""},
		{"no body", nil, http.StatusOK, ""},
		// "Сливки" в кодировке Windows-1251: так тело уходит из консоли Windows
		{"not utf-8", []byte{'"', 0xd1, 0xeb, 0xe8, 0xe2, 0xea, 0xe8, '"'}, http.StatusBadRequest, "Тело запроса содержит некорректный UTF-8"},
		{"too large", bytes.Repeat([]byte("a"), 17), http.StatusRequestEntityTooLarge, "Тело запроса слишком большое"},
		// нулевой символ проходит UTF-8 и JSON, но PostgreSQL его не хранит: раньше это был ответ 500
		{"escaped NUL", []byte(`{"n":"a\u0000b"}`), http.StatusBadRequest, "Тело запроса содержит нулевой символ"},
		{"raw NUL", []byte{'"', 'a', 0, '"'}, http.StatusBadRequest, "Тело запроса содержит нулевой символ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got = nil
			var body io.Reader
			if tt.body != nil {
				body = bytes.NewReader(tt.body)
			}

			w := serve(h, httptest.NewRequest(http.MethodPost, "/", body))

			require.Equal(t, tt.status, w.Code)
			if tt.status == http.StatusOK {
				require.Equal(t, string(tt.body), string(got), "the handler reads the same body")
				return
			}
			require.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
			var problem httpx.Problem
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
			require.Equal(t, "bad-request", problem.Type)
			require.Equal(t, tt.wantTitle, problem.Title)
			require.Nil(t, got, "the handler is not called")
		})
	}

	t.Run("unreadable", func(t *testing.T) {
		w := serve(h, httptest.NewRequest(http.MethodPost, "/", unreadableBody{}))

		require.Equal(t, http.StatusBadRequest, w.Code)
		var problem httpx.Problem
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
		require.Equal(t, "bad-request", problem.Type)
		require.Equal(t, "Не удалось прочитать тело запроса", problem.Title)
	})
}

func TestActorInContext(t *testing.T) {
	want := eventstore.Actor{Type: eventstore.ActorUser, ID: "approver"}
	actor, found := httpx.Actor(httpx.WithActor(context.Background(), want))
	require.True(t, found)
	require.Equal(t, want, actor)

	_, found = httpx.Actor(context.Background())
	require.False(t, found)
}

func TestCORS(t *testing.T) {
	const allowed = "http://localhost:5173"
	called := false
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}), httpx.CORS([]string{allowed}))

	request := func(method, origin, requestMethod string) *httptest.ResponseRecorder {
		called = false
		r := httptest.NewRequest(method, "/api/v1/products", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if requestMethod != "" {
			r.Header.Set("Access-Control-Request-Method", requestMethod)
		}
		return serve(h, r)
	}

	t.Run("preflight from an allowed origin", func(t *testing.T) {
		w := request(http.MethodOptions, allowed, http.MethodPost)
		require.Equal(t, http.StatusNoContent, w.Code)
		require.False(t, called, "preflight is answered without the handler, so without a token")
		require.Equal(t, allowed, w.Header().Get("Access-Control-Allow-Origin"))
		require.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
		// Last-Event-ID отправляет браузерный клиент SSE при переподключении
		for _, header := range []string{"Authorization", "Content-Type", "Idempotency-Key", "Last-Event-ID", httpx.CorrelationHeader} {
			require.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), header)
		}
	})
	t.Run("request from an allowed origin", func(t *testing.T) {
		w := request(http.MethodGet, allowed, "")
		require.True(t, called)
		require.Equal(t, http.StatusTeapot, w.Code)
		require.Equal(t, allowed, w.Header().Get("Access-Control-Allow-Origin"))
	})
	t.Run("foreign origin", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodOptions} {
			w := request(method, "http://evil.example", http.MethodPost)
			require.True(t, called)
			require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			require.Empty(t, w.Header().Get("Access-Control-Allow-Headers"))
		}
	})
	t.Run("no origin", func(t *testing.T) {
		w := request(http.MethodGet, "", "")
		require.True(t, called)
		require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestHealth(t *testing.T) {
	var ready error
	h := httpx.Health(func(context.Context) error { return ready })

	require.Equal(t, http.StatusOK, serve(h, httptest.NewRequest(http.MethodGet, "/healthz", nil)).Code)
	require.Equal(t, http.StatusOK, serve(h, httptest.NewRequest(http.MethodGet, "/readyz", nil)).Code)

	ready = errors.New("database is down")
	require.Equal(t, http.StatusOK, serve(h, httptest.NewRequest(http.MethodGet, "/healthz", nil)).Code,
		"the process is alive even when it is not ready")
	require.Equal(t, http.StatusServiceUnavailable, serve(h, httptest.NewRequest(http.MethodGet, "/readyz", nil)).Code)
}

// Запрос, принятый до сигнала завершения, дорабатывает; новые соединения не принимаются.
func TestServeFinishesRequestInFlight(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	url := "http://" + listener.Addr().String()

	entered, release := make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("done"))
	})

	ctx, stop := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- httpx.ServeListener(ctx, listener, handler, 5*time.Second, quiet) }()

	type result struct {
		body string
		err  error
	}
	response := make(chan result, 1)
	go func() {
		resp, err := http.Get(url) //nolint:noctx // тест
		if err != nil {
			response <- result{err: err}
			return
		}
		defer resp.Body.Close() //nolint:errcheck
		body, err := io.ReadAll(resp.Body)
		response <- result{string(body), err}
	}()

	<-entered
	stop() // сигнал завершения во время обработки запроса

	select {
	case err := <-served:
		t.Fatalf("server stopped before the request finished: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	got := <-response
	require.NoError(t, got.err)
	require.Equal(t, "done", got.body)
	require.NoError(t, <-served)

	_, err = http.Get(url) //nolint:noctx // тест
	require.Error(t, err, "no new connections after shutdown")
}

// Паника в обработчике попадает в журнал запросов со статусом 500: Recover стоит внутри
// AccessLog. Снаружи он перехватывал панику уже после того, как журнал её пропустил.
func TestAccessLogSeesRecoveredPanic(t *testing.T) {
	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, nil))
	h := httpx.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
		httpx.AccessLog(logger), httpx.Recover(logger))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, logged.String(), `"msg":"http request"`)
	require.Contains(t, logged.String(), `"status":500`)
}
