package redis

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/config"
)

type fakeStore struct {
	mu      sync.Mutex
	values  map[string][]byte
	expires map[string]time.Time
	getErr  error
	setErr  error
}

func (f *fakeStore) Get(_ context.Context, key string) *goredis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return goredis.NewStringResult("", f.getErr)
	}
	if expiresAt, ok := f.expires[key]; ok && time.Now().After(expiresAt) {
		delete(f.values, key)
		delete(f.expires, key)
	}
	value, ok := f.values[key]
	if !ok {
		return goredis.NewStringResult("", goredis.Nil)
	}
	return goredis.NewStringResult(string(value), nil)
}

func (f *fakeStore) Set(_ context.Context, key string, value any, ttl time.Duration) *goredis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return goredis.NewStatusResult("", f.setErr)
	}
	f.values[key] = append([]byte(nil), value.([]byte)...)
	if f.expires == nil {
		f.expires = map[string]time.Time{}
	}
	f.expires[key] = time.Now().Add(ttl)
	return goredis.NewStatusResult("OK", nil)
}

func (*fakeStore) Close() error { return nil }

func testCache(client store) *Cache {
	return newCache(client, time.Minute, time.Second, slog.New(slog.DiscardHandler))
}

func TestDisabledCacheBypassesRedis(t *testing.T) {
	cache := testCache(nil)
	value, err := LoadJSON(context.Background(), cache, "key", func(context.Context) (int, error) { return 42, nil })
	require.NoError(t, err)
	require.Equal(t, 42, value)
}

func TestCacheMissThenHit(t *testing.T) {
	cache := testCache(&fakeStore{values: map[string][]byte{}})
	var loads int
	loader := func(context.Context) (int, error) { loads++; return 42, nil }

	first, err := LoadJSON(context.Background(), cache, "key", loader)
	require.NoError(t, err)
	second, err := LoadJSON(context.Background(), cache, "key", loader)
	require.NoError(t, err)
	require.Equal(t, 42, first)
	require.Equal(t, first, second)
	require.Equal(t, 1, loads)
}

func TestRedisErrorsFallBackToLoader(t *testing.T) {
	cache := testCache(&fakeStore{values: map[string][]byte{}, getErr: errors.New("offline"), setErr: errors.New("offline")})
	value, err := LoadJSON(context.Background(), cache, "key", func(context.Context) (int, error) { return 42, nil })
	require.NoError(t, err)
	require.Equal(t, 42, value)
}

func TestConcurrentMissesShareLoader(t *testing.T) {
	cache := testCache(&fakeStore{values: map[string][]byte{}})
	var loads atomic.Int32
	release := make(chan struct{})
	loader := func(context.Context) (int, error) {
		loads.Add(1)
		<-release
		return 42, nil
	}

	const callers = 8
	values := make(chan int, callers)
	errs := make(chan error, callers)
	for range callers {
		go func() {
			value, err := LoadJSON(context.Background(), cache, "key", loader)
			values <- value
			errs <- err
		}()
	}
	for loads.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(release)
	for range callers {
		require.NoError(t, <-errs)
		require.Equal(t, 42, <-values)
	}
	require.EqualValues(t, 1, loads.Load())
}

func TestExpiredValueIsLoadedAgain(t *testing.T) {
	cache := newCache(&fakeStore{values: map[string][]byte{}}, 5*time.Millisecond, time.Second, slog.New(slog.DiscardHandler))
	var loads int
	loader := func(context.Context) (int, error) { loads++; return loads, nil }

	first, err := LoadJSON(context.Background(), cache, "key", loader)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	second, err := LoadJSON(context.Background(), cache, "key", loader)
	require.NoError(t, err)
	require.Equal(t, 1, first)
	require.Equal(t, 2, second)
}

func TestNewValidatesConfiguration(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	_, err := New(config.Redis{CacheTTL: time.Minute, Timeout: time.Second, URL: "://bad"}, logger, nil)
	require.ErrorContains(t, err, "REDIS_URL")
	_, err = New(config.Redis{CacheTTL: 0, Timeout: time.Second}, logger, nil)
	require.ErrorContains(t, err, "REDIS_CACHE_TTL")
	_, err = New(config.Redis{CacheTTL: time.Minute, Timeout: 0}, logger, nil)
	require.ErrorContains(t, err, "REDIS_TIMEOUT")
}

// В кеше лежит корректный JSON другой формы (например, от другой версии кода): ответ
// читается из источника, а не завершается ошибкой до истечения срока хранения.
func TestCachedValueOfAnotherShapeFallsBackToLoader(t *testing.T) {
	cache := testCache(&fakeStore{values: map[string][]byte{"key": []byte(`{"unexpected":"object"}`)}})

	value, err := LoadJSON(context.Background(), cache, "key", func(context.Context) (int, error) { return 42, nil })

	require.NoError(t, err)
	require.Equal(t, 42, value)
}

// Первый клиент отключился, пока шла общая загрузка: остальные, кто ждёт тот же ключ,
// не должны получить его ошибку отмены.
func TestSharedLoadDoesNotDependOnTheFirstCaller(t *testing.T) {
	cache := testCache(&fakeStore{values: map[string][]byte{}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	value, err := LoadJSON(ctx, cache, "key", func(ctx context.Context) (int, error) {
		return 42, ctx.Err() // загрузка идёт в контексте, не зависящем от отмены вызвавшего
	})

	require.NoError(t, err)
	require.Equal(t, 42, value)
}
