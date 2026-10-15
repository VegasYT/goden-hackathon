package realtime

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandlerStartsWithRetryAndSync(t *testing.T) {
	body, header, status := serveUntilFlush(t, http.Header{"Last-Event-ID": {uuid.Must(uuid.NewV7()).String()}}, nil)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "text/event-stream", header.Get("Content-Type"))
	require.Equal(t, "no-cache", header.Get("Cache-Control"))
	require.Contains(t, body, "retry: 3000\n")
	require.Contains(t, body, "event: sync\n")
	require.Contains(t, body, `"categories":["catalog","deliveries","expiry","orders","recommendations","stock","stop-list"]`)
}

func TestHandlerWritesUpdate(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	w := newStreamWriter(cancel, 2)
	done := make(chan struct{})
	go func() {
		newHandler(hub, time.Hour, defaultRetry).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(WithScope(ctx, scope)))
		close(done)
	}()
	<-w.flushed
	update := Update{ID: uuid.Must(uuid.NewV7()), PointID: scope.PointID, OccurredAt: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC), Categories: []Category{CategoryStock}}
	hub.Publish(update)
	<-w.flushed
	<-done

	body := w.bodyString()
	require.Contains(t, body, "id: "+update.ID.String()+"\n")
	require.Contains(t, body, "event: update\n")
	require.Contains(t, body, `"categories":["stock"]`)
}

func TestHandlerKeepsIdleConnectionAlive(t *testing.T) {
	body, _, _ := serveUntilFlush(t, nil, func(hub *Hub) http.Handler {
		return newHandler(hub, time.Millisecond, defaultRetry)
	})
	require.Contains(t, body, ": keepalive\n\n")
}

func serveUntilFlush(t *testing.T, headers http.Header, factory func(*Hub) http.Handler) (string, http.Header, int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	flushes := 1
	if factory != nil {
		flushes = 2
	}
	w := newStreamWriter(cancel, flushes)
	request := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(WithScope(ctx, scope))
	for name, values := range headers {
		request.Header[name] = values
	}
	hub := NewHub()
	handler := NewHandler(hub)
	if factory != nil {
		handler = factory(hub)
	}
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, request)
		close(done)
	}()
	for range flushes {
		<-w.flushed
	}
	<-done
	return w.bodyString(), w.header.Clone(), w.status
}

type streamWriter struct {
	mu       sync.Mutex
	header   http.Header
	body     bytes.Buffer
	status   int
	flushes  int
	cancelAt int
	cancel   context.CancelFunc
	flushed  chan struct{}
}

func newStreamWriter(cancel context.CancelFunc, cancelAt int) *streamWriter {
	return &streamWriter{header: make(http.Header), cancel: cancel, cancelAt: cancelAt, flushed: make(chan struct{}, cancelAt)}
}

func (w *streamWriter) Header() http.Header { return w.header }

func (w *streamWriter) WriteHeader(status int) { w.status = status }

func (w *streamWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.Write(data)
}

func (w *streamWriter) Flush() {
	w.flushes++
	w.flushed <- struct{}{}
	if w.flushes == w.cancelAt {
		w.cancel()
	}
}

func (w *streamWriter) bodyString() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.ReplaceAll(w.body.String(), "\r\n", "\n")
}

// Сервер с подключённым клиентом останавливается сразу после закрытия hub: без него
// http.Server.Shutdown ждал бы открытый поток до истечения срока и завершался ошибкой.
func TestServerShutsDownWithConnectedClient(t *testing.T) {
	hub := NewHub()
	server := httptest.NewServer(NewHandler(hub))
	defer server.Close()

	response, err := http.Get(server.URL) //nolint:noctx
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck
	buffer := make([]byte, 64)
	_, err = response.Body.Read(buffer) // поток открыт: пришли retry и sync
	require.NoError(t, err)

	hub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := time.Now()

	require.NoError(t, server.Config.Shutdown(ctx), "shutdown does not wait for the stream")
	require.Less(t, time.Since(started), 2*time.Second)
}

// Сигналы, пришедшие подряд, уходят клиенту одним: категории объединены, идентификатор последнего.
func TestHandlerMergesUpdatesThatComeTogether(t *testing.T) {
	h := &Handler{merge: 50 * time.Millisecond}
	updates := make(chan Update, 4)
	first := Update{ID: uuid.New(), Categories: []Category{CategoryStock}}
	last := Update{ID: uuid.New(), Categories: []Category{CategoryExpiry}}
	updates <- Update{ID: uuid.New(), Categories: []Category{CategoryStopList, CategoryStock}}
	updates <- last

	merged, open := h.collect(context.Background(), updates, first)
	require.True(t, open)
	require.Equal(t, last.ID, merged.ID)
	require.Equal(t, []Category{CategoryExpiry, CategoryStock, CategoryStopList}, merged.Categories)

	// подписку закрыли во время окна: накопленное отдаётся, поток после этого завершается
	updates <- last
	close(updates)
	merged, open = h.collect(context.Background(), updates, first)
	require.False(t, open)
	require.Equal(t, last.ID, merged.ID)

	// без окна сигнал уходит как есть
	merged, open = (&Handler{}).collect(context.Background(), make(chan Update), first)
	require.True(t, open)
	require.Equal(t, first, merged)
}
