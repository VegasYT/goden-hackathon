//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability/metricstest"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

// countingProducer считает публикации по идентификатору события; им пользуются несколько publisher сразу.
type countingProducer struct {
	mu     sync.Mutex
	counts map[uuid.UUID]int
}

func (p *countingProducer) Produce(_ context.Context, message Message) error {
	var event Envelope
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.counts == nil {
		p.counts = map[uuid.UUID]int{}
	}
	p.counts[event.EventID]++
	return nil
}

func (p *countingProducer) snapshot() map[uuid.UUID]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[uuid.UUID]int, len(p.counts))
	for id, n := range p.counts {
		out[id] = n
	}
	return out
}

// appendStockEvents добавляет n событий, которые publisher публикует, и возвращает их идентификаторы.
func appendStockEvents(t *testing.T, pool *pgxpool.Pool, stream string, n int) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	ids := make([]uuid.UUID, n)
	require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return err
		}
		events := make([]eventstore.NewEvent, n)
		for i := range events {
			ids[i] = uuid.New()
			events[i] = eventstore.NewEvent{
				ID: ids[i], Type: "BatchReceived", SchemaVersion: 1,
				Payload: json.RawMessage(`{}`), OccurredAt: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
			}
		}
		_, err = eventstore.Append(ctx, tx, stream, version, events...)
		return err
	}))
	return ids
}

// runPublisher запускает publisher в фоне и возвращает функцию остановки, которая дожидается выхода.
func runPublisher(t *testing.T, pool *pgxpool.Pool, producer Producer, registry prometheus.Registerer) (stop func()) {
	t.Helper()
	publisher := NewPublisher(pool, producer, clock.NewFake(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)),
		slog.New(slog.DiscardHandler), registry, "goden")
	publisher.PollInterval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- publisher.Run(ctx) }()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			require.NoError(t, <-done)
		})
	}
	t.Cleanup(stop)
	return stop
}

// Два экземпляра на одной базе: публикует один, каждое событие уходит один раз. После
// остановки публикующего второй продолжает с сохранённой позиции.
func TestOnlyOnePublisherPublishes(t *testing.T) {
	pool := pgtest.NewDB(t)
	producer := &countingProducer{}
	stopFirst := runPublisher(t, pool, producer, prometheus.NewRegistry())

	first := appendStockEvents(t, pool, "stock-a", 5)
	require.Eventually(t, func() bool { return len(producer.snapshot()) == 5 }, 20*time.Second, 20*time.Millisecond,
		"the first publisher takes the lock and publishes")

	runPublisher(t, pool, producer, prometheus.NewRegistry())
	second := appendStockEvents(t, pool, "stock-b", 20)
	require.Eventually(t, func() bool { return len(producer.snapshot()) == 25 }, 20*time.Second, 20*time.Millisecond)

	stopFirst()
	third := appendStockEvents(t, pool, "stock-c", 5)
	require.Eventually(t, func() bool { return len(producer.snapshot()) == 30 }, 20*time.Second, 20*time.Millisecond,
		"the waiting publisher takes over")

	time.Sleep(300 * time.Millisecond) // дать время возможной повторной публикации
	counts := producer.snapshot()
	for _, id := range append(append(first, second...), third...) {
		require.Equal(t, 1, counts[id], "event %s", id)
	}
}

// Kafka не подтверждает записи: сбой виден в метриках, позиция не двигается.
func TestPublisherFailureIsVisibleInMetrics(t *testing.T) {
	pool := pgtest.NewDB(t)
	registry := prometheus.NewRegistry()
	appendStockEvents(t, pool, "stock-a", 4)
	runPublisher(t, pool, &fakeProducer{err: errors.New("kafka: produce: context deadline exceeded")}, registry)

	require.Eventually(t, func() bool {
		return metricstest.Value(t, registry, "goden_kafka_publisher_errors_total") > 0 &&
			metricstest.Value(t, registry, "goden_kafka_publisher_lag_events") == 4
	}, 20*time.Second, 20*time.Millisecond)

	var checkpoints int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM kafka_publisher_checkpoints`).Scan(&checkpoints))
	require.Zero(t, checkpoints, "the position does not move without an acknowledgement")
}

func stockRecord(t *testing.T, eventID uuid.UUID) *kgo.Record {
	t.Helper()
	raw, err := json.Marshal(Envelope{
		EnvelopeVersion: EnvelopeVersion,
		EventID:         eventID, EventType: "StockConsumed", EventVersion: 1,
		StreamID: "stock", StreamVersion: 1, Payload: json.RawMessage(`{}`),
		Metadata: json.RawMessage(`{}`), OccurredAt: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	return &kgo.Record{Topic: "goden.stock.v1", Value: raw}
}

// Обработка всегда завершается ошибкой: после предельного числа попыток событие уходит
// в поток ошибок и больше не обрабатывается, раздел не стоит без конца.
func TestConsumerGivesUpAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	producer := &fakeProducer{}
	calls := 0
	consumer := testConsumer(t, func(context.Context, pgx.Tx, Envelope) error {
		calls++
		return fmt.Errorf("attempt %d failed", calls)
	}, producer)
	consumer.MaxAttempts = 3
	eventID := uuid.New()

	require.NoError(t, consumer.processWithRetry(ctx, stockRecord(t, eventID)))

	require.Equal(t, 3, calls)
	require.Len(t, producer.messages, 1)
	require.Equal(t, "goden.errors.v1", producer.messages[0].Topic)
	var failure ErrorEnvelope
	require.NoError(t, json.Unmarshal(producer.messages[0].Value, &failure))
	require.Equal(t, eventID, failure.EventID)
	require.Contains(t, failure.Error, "gave up after 3 attempts")
	require.Contains(t, failure.Error, "attempt 3 failed")

	// событие отмечено: повторная доставка обработчик не вызывает
	outcome, err := consumer.Process(ctx, "goden.stock.v1", stockRecord(t, eventID).Value)
	require.NoError(t, err)
	require.Equal(t, "duplicate", outcome)
	require.Equal(t, 3, calls)
}

func TestConsumerRetriesTemporaryFailure(t *testing.T) {
	producer := &fakeProducer{}
	calls := 0
	consumer := testConsumer(t, func(context.Context, pgx.Tx, Envelope) error {
		if calls++; calls < 3 {
			return errors.New("temporary")
		}
		return nil
	}, producer)
	consumer.MaxAttempts = 5

	require.NoError(t, consumer.processWithRetry(context.Background(), stockRecord(t, uuid.New())))

	require.Equal(t, 3, calls)
	require.Empty(t, producer.messages, "an event handled on a retry does not reach the error stream")
}

// Kafka недоступна при запуске: задача не возвращает ошибку (она остановила бы весь worker),
// а ждёт и завершается только вместе с контекстом.
func TestConsumerRunSurvivesUnavailableKafka(t *testing.T) {
	registry := prometheus.NewRegistry()
	consumer, err := NewConsumer(pgtest.NewDB(t), Config{Brokers: []string{"127.0.0.1:1"}, TopicPrefix: "goden"},
		"unavailable", []string{StockTopic}, func(context.Context, pgx.Tx, Envelope) error { return nil },
		clock.NewFake(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)), slog.New(slog.DiscardHandler), registry)
	require.NoError(t, err)
	t.Cleanup(consumer.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	started := time.Now()

	require.NoError(t, consumer.Run(ctx))
	require.GreaterOrEqual(t, time.Since(started), 3*time.Second, "the task keeps retrying until the context is cancelled")
}
