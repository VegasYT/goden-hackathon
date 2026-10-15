// Package redis предоставляет необязательный Redis-кеш для восстанавливаемых read-моделей.
package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/projection"
)

const disabledResult = "bypass"

type store interface {
	Get(context.Context, string) *goredis.StringCmd
	Set(context.Context, string, any, time.Duration) *goredis.StatusCmd
	Close() error
}

// Cache хранит только восстанавливаемые JSON-ответы. Ошибка Redis всегда приводит
// к загрузке из PostgreSQL, а не к ошибке пользовательского запроса.
type Cache struct {
	client       store
	ttl          time.Duration
	timeout      time.Duration
	logger       *slog.Logger
	checkpoints  eventstore.Querier
	group        singleflight.Group
	logMu        sync.Mutex
	lastErrorLog time.Time

	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New создаёт кеш. Пустой REDIS_URL возвращает отключённый кеш без сетевых вызовов.
func New(cfg config.Redis, logger *slog.Logger, checkpoints eventstore.Querier) (*Cache, error) {
	if logger == nil {
		return nil, errors.New("redis cache: logger is required")
	}
	if cfg.CacheTTL <= 0 {
		return nil, errors.New("redis cache: REDIS_CACHE_TTL must be positive")
	}
	if cfg.Timeout <= 0 {
		return nil, errors.New("redis cache: REDIS_TIMEOUT must be positive")
	}
	cache := newCache(nil, cfg.CacheTTL, cfg.Timeout, logger)
	if cfg.URL == "" {
		return cache, nil
	}
	options, err := goredis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redis cache: REDIS_URL is invalid: %w", err)
	}
	if checkpoints == nil {
		return nil, errors.New("redis cache: checkpoint reader is required")
	}
	options.DialTimeout = cfg.Timeout
	options.ReadTimeout = cfg.Timeout
	options.WriteTimeout = cfg.Timeout
	cache.client = goredis.NewClient(options)
	cache.checkpoints = checkpoints
	return cache, nil
}

func newCache(client store, ttl, timeout time.Duration, logger *slog.Logger) *Cache {
	return &Cache{
		client: client, ttl: ttl, timeout: timeout, logger: logger,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "read_cache_requests_total",
			Help: "Read-cache requests by result: hit, miss, error, bypass.",
		}, []string{"result"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: observability.MetricsNamespace, Name: "read_cache_operation_duration_seconds",
			Help: "Duration of Redis read-cache operations.",
		}, []string{"operation"}),
	}
}

// Register добавляет метрики кеша в реестр процесса.
func (c *Cache) Register(registry prometheus.Registerer) {
	for _, result := range []string{"hit", "miss", "error", disabledResult} {
		c.requests.WithLabelValues(result)
	}
	for _, operation := range []string{"get", "set"} {
		c.duration.WithLabelValues(operation)
	}
	registry.MustRegister(c.requests, c.duration)
}

// Enabled сообщает, настроено ли подключение к Redis.
func (c *Cache) Enabled() bool { return c != nil && c.client != nil }

// Close освобождает соединения клиента.
func (c *Cache) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.client.Close()
}

type loaded struct {
	raw    []byte
	result string
}

// LoadJSON возвращает значение из Redis либо вызывает loader. Одновременные промахи
// одного экземпляра объединяются; каждый вызывающий запрос учитывается в метрике.
func LoadJSON[T any](ctx context.Context, cache *Cache, key string, loader func(context.Context) (T, error)) (T, error) {
	if cache == nil || !cache.Enabled() {
		if cache != nil {
			cache.requests.WithLabelValues(disabledResult).Inc()
		}
		return loader(ctx)
	}

	// Общий запрос не должен зависеть от отмены первого клиента: остальные, кто ждёт тот же ключ,
	// получили бы его ошибку отмены, хотя их запросы живы.
	shared := context.WithoutCancel(ctx)
	value, err, _ := cache.group.Do(key, func() (any, error) {
		return cache.load(shared, key, func(ctx context.Context) ([]byte, error) {
			value, err := loader(ctx)
			if err != nil {
				return nil, err
			}
			return json.Marshal(value)
		})
	})
	if err != nil {
		var zero T
		return zero, err
	}
	entry := value.(loaded)
	cache.requests.WithLabelValues(entry.result).Inc()
	var result T
	if err := json.Unmarshal(entry.raw, &result); err != nil {
		if entry.result == "hit" {
			// в кеше значение другой формы (например, от другой версии кода): читаем из источника
			cache.warn(ctx, "redis cache value has an unexpected shape", err)
			return loader(ctx)
		}
		return result, fmt.Errorf("redis cache: decode loaded value: %w", err)
	}
	return result, nil
}

// LoadProjectionJSON кеширует результат под версией и позицией PostgreSQL-проекции.
// Если checkpoint ещё не создан, результат читается напрямую и не сохраняется.
func LoadProjectionJSON[T any](ctx context.Context, cache *Cache, projectionName, operation string, params any,
	loader func(context.Context) (T, error),
) (T, error) {
	if cache == nil || !cache.Enabled() {
		return LoadJSON(ctx, cache, "", loader)
	}
	state, found, err := projection.ReadCheckpoint(ctx, cache.checkpoints, projectionName)
	if err != nil {
		var zero T
		return zero, err
	}
	if !found {
		cache.requests.WithLabelValues(disabledResult).Inc()
		return loader(ctx)
	}
	raw, err := json.Marshal(params)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("redis cache: encode key parameters: %w", err)
	}
	hash := sha256.Sum256(raw)
	key := "goden:v1:" + projectionName + ":" + strconv.Itoa(state.Version) + ":" +
		strconv.FormatUint(state.TxID, 10) + ":" + strconv.FormatInt(state.Position, 10) + ":" +
		operation + ":" + hex.EncodeToString(hash[:])
	return LoadJSON(ctx, cache, key, loader)
}

func (c *Cache) load(ctx context.Context, key string, loader func(context.Context) ([]byte, error)) (loaded, error) {
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	started := time.Now()
	raw, err := c.client.Get(opCtx, key).Bytes()
	c.duration.WithLabelValues("get").Observe(time.Since(started).Seconds())
	cancel()
	if err == nil {
		if json.Valid(raw) {
			return loaded{raw: raw, result: "hit"}, nil
		}
		err = errors.New("cached value is not valid JSON")
	}

	result := "miss"
	if err != nil && !errors.Is(err, goredis.Nil) {
		result = "error"
		c.warn(ctx, "redis cache read failed", err)
	}
	raw, loadErr := loader(ctx)
	if loadErr != nil {
		return loaded{}, loadErr
	}

	opCtx, cancel = context.WithTimeout(ctx, c.timeout)
	started = time.Now()
	setErr := c.client.Set(opCtx, key, raw, c.ttl).Err()
	c.duration.WithLabelValues("set").Observe(time.Since(started).Seconds())
	cancel()
	if setErr != nil {
		result = "error"
		c.warn(ctx, "redis cache write failed", setErr)
	}
	return loaded{raw: raw, result: result}, nil
}

func (c *Cache) warn(ctx context.Context, message string, err error) {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	now := time.Now()
	if !c.lastErrorLog.IsZero() && now.Sub(c.lastErrorLog) < time.Minute {
		return
	}
	c.lastErrorLog = now
	c.logger.WarnContext(ctx, message, "error", err)
}
