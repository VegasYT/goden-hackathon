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
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

type fakeProducer struct {
	messages []Message
	err      error
}

func (p *fakeProducer) Produce(_ context.Context, message Message) error {
	if p.err != nil {
		return p.err
	}
	p.messages = append(p.messages, message)
	return nil
}

func TestPublisherRetriesAfterFailureAndRestart(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	eventID := uuid.New()
	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := eventstore.Append(ctx, tx, "stock-product", 0, eventstore.NewEvent{
			ID: eventID, Type: "BatchReceived", SchemaVersion: 1,
			Payload: json.RawMessage(`{"batch_id":"b"}`), OccurredAt: now,
		})
		return err
	})
	require.NoError(t, err)

	failed := &fakeProducer{err: errors.New("broker unavailable")}
	first := NewPublisher(pool, failed, clock.NewFake(now), slog.New(slog.DiscardHandler),
		observability.NewRegistry(), "goden")
	_, err = first.PublishBatch(ctx)
	require.ErrorContains(t, err, "broker unavailable")
	var checkpoints int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM kafka_publisher_checkpoints`).Scan(&checkpoints))
	require.Zero(t, checkpoints, "failed Kafka ack must not advance the checkpoint")

	recovered := &fakeProducer{}
	second := NewPublisher(pool, recovered, clock.NewFake(now), slog.New(slog.DiscardHandler),
		observability.NewRegistry(), "goden")
	published, err := second.PublishBatch(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, published)
	require.Len(t, recovered.messages, 1)
	require.Equal(t, "goden.stock.v1", recovered.messages[0].Topic)

	var envelope Envelope
	require.NoError(t, json.Unmarshal(recovered.messages[0].Value, &envelope))
	require.Equal(t, eventID, envelope.EventID)

	published, err = second.PublishBatch(ctx)
	require.NoError(t, err)
	require.Zero(t, published)
	require.Len(t, recovered.messages, 1, "saved checkpoint prevents a second publish after restart")
}

func TestPublisherSkipsInternalEventsAndAdvancesCheckpoint(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := eventstore.Append(ctx, tx, "product", 0, eventstore.NewEvent{
			ID: uuid.New(), Type: "InternalEvent", SchemaVersion: 1,
			Payload: json.RawMessage(`{}`), OccurredAt: now,
		})
		return err
	})
	require.NoError(t, err)

	producer := &fakeProducer{}
	publisher := NewPublisher(pool, producer, clock.NewFake(now), slog.New(slog.DiscardHandler),
		observability.NewRegistry(), "goden")
	inspected, err := publisher.PublishBatch(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, inspected)
	require.Empty(t, producer.messages)

	inspected, err = publisher.PublishBatch(ctx)
	require.NoError(t, err)
	require.Zero(t, inspected)
}
