//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/twmb/franz-go/pkg/kgo"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/dockertest"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres/pgtest"
)

func testConsumer(t *testing.T, handler Handler, producer Producer) *Consumer {
	t.Helper()
	return &Consumer{
		pool: pgtest.NewDB(t), producer: producer, handler: handler,
		clock:  clock.NewFake(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)),
		logger: slog.New(slog.DiscardHandler), name: "test", prefix: "goden",
		processed: observability.Register(observability.NewRegistry(), prometheusCounterVec()),
		failures:  observability.Register(observability.NewRegistry(), prometheusCounter()),
	}
}

// Маленькие конструкторы сохраняют production-набор labels в тестовом Consumer.
func prometheusCounterVec() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_consumer_messages_total", Help: "test"}, []string{"consumer", "outcome"})
}

func prometheusCounter() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{Name: "test_consumer_errors_total", Help: "test"})
}

func TestConsumerHandlesEventOnlyOnce(t *testing.T) {
	ctx := context.Background()
	producer := &fakeProducer{}
	consumer := testConsumer(t, func(ctx context.Context, tx pgx.Tx, event Envelope) error {
		_, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS consumer_effects (event_id uuid PRIMARY KEY)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO consumer_effects (event_id) VALUES ($1)`, event.EventID)
		return err
	}, producer)
	event := Envelope{
		EnvelopeVersion: EnvelopeVersion,
		EventID:         uuid.New(), EventType: "StockConsumed", EventVersion: 1,
		StreamID: "stock", StreamVersion: 1, Payload: json.RawMessage(`{}`),
		Metadata: json.RawMessage(`{}`), OccurredAt: consumer.clock.Now(),
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)

	outcome, err := consumer.Process(ctx, "goden.stock.v1", raw)
	require.NoError(t, err)
	require.Equal(t, "handled", outcome)
	outcome, err = consumer.Process(ctx, "goden.stock.v1", raw)
	require.NoError(t, err)
	require.Equal(t, "duplicate", outcome)

	var effects int
	require.NoError(t, consumer.pool.QueryRow(ctx, `SELECT count(*) FROM consumer_effects`).Scan(&effects))
	require.Equal(t, 1, effects)
	require.Empty(t, producer.messages)
}

func TestConsumerSendsPermanentFailureToErrorStream(t *testing.T) {
	ctx := context.Background()
	producer := &fakeProducer{}
	consumer := testConsumer(t, func(context.Context, pgx.Tx, Envelope) error {
		return Permanent(errors.New("unsupported payload"))
	}, producer)
	event := Envelope{
		EnvelopeVersion: EnvelopeVersion,
		EventID:         uuid.New(), EventType: "StockConsumed", EventVersion: 1,
		StreamID: "stock", StreamVersion: 1, Payload: json.RawMessage(`{}`),
		Metadata: json.RawMessage(`{}`), OccurredAt: consumer.clock.Now(),
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)

	outcome, err := consumer.Process(ctx, "goden.stock.v1", raw)
	require.NoError(t, err)
	require.Equal(t, "error-stream", outcome)
	require.Len(t, producer.messages, 1)
	require.Equal(t, "goden.errors.v1", producer.messages[0].Topic)

	// После записи outcome повтор доставляется, но handler и error stream не вызываются снова.
	consumer.handler = func(context.Context, pgx.Tx, Envelope) error {
		t.Fatal("handler called for an event already stored in the inbox")
		return nil
	}
	outcome, err = consumer.Process(ctx, "goden.stock.v1", raw)
	require.NoError(t, err)
	require.Equal(t, "duplicate", outcome)
	require.Len(t, producer.messages, 1)
}

func TestConsumerCreatesTopicsBeforePolling(t *testing.T) {
	dockertest.Setup()
	containerCtx, cancelContainer := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelContainer()
	container, err := tckafka.Run(containerCtx, "confluentinc/confluent-local:7.5.0",
		testcontainers.WithEnv(map[string]string{"KAFKA_AUTO_CREATE_TOPICS_ENABLE": "false"}))
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)
	brokers, err := container.Brokers(containerCtx)
	require.NoError(t, err)

	pool := pgtest.NewDB(t)
	handled := make(chan struct{}, 1)
	cfg := Config{Brokers: brokers, TopicPrefix: "consumer-topic-test"}
	consumer, err := NewConsumer(pool, cfg, "topic-test", []string{ExpiryTopic},
		func(context.Context, pgx.Tx, Envelope) error {
			handled <- struct{}{}
			return nil
		}, clock.NewFake(time.Now()), slog.New(slog.DiscardHandler), observability.NewRegistry())
	require.NoError(t, err)
	t.Cleanup(consumer.Close)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)
	raw, err := json.Marshal(Envelope{
		EnvelopeVersion: EnvelopeVersion, EventID: uuid.New(), EventType: "BatchExpiryWarningIssued",
		EventVersion: 1, StreamID: "stock", StreamVersion: 1,
		Payload: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`), OccurredAt: time.Now(),
	})
	require.NoError(t, err)
	topic := FullTopic(cfg.TopicPrefix, ExpiryTopic)

	require.Eventually(t, func() bool {
		result := producer.ProduceSync(context.Background(), &kgo.Record{Topic: topic, Value: raw})
		return result.FirstErr() == nil
	}, 30*time.Second, 200*time.Millisecond, "consumer did not explicitly create %s", topic)

	select {
	case <-handled:
	case <-time.After(20 * time.Second):
		t.Fatal("consumer did not read from the explicitly created topic")
	}
}
