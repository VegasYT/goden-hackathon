//go:build integration

package notifications_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/notifications"
	"goden/backend/internal/platform/clock"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/platform/postgres/pgtest"
)

func TestErrorReaderDeduplicatesAndSkipsMalformed(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	reader, err := notifications.NewErrorReader(
		pool, platformkafka.Config{Brokers: []string{"localhost:1"}, TopicPrefix: "test"},
		notifications.NewQueue(pool), clock.NewFake(now), slog.New(slog.DiscardHandler), prometheus.NewRegistry(),
	)
	require.NoError(t, err)
	t.Cleanup(reader.Close)

	raw, err := json.Marshal(platformkafka.ErrorEnvelope{
		Consumer: "analytics", SourceTopic: "test.stock.v1", EventID: uuid.New(),
		EventType: "StockWrittenOff", Error: "unsupported payload", Original: []byte(`{"secret":true}`), FailedAt: now,
	})
	require.NoError(t, err)

	for range 2 {
		outcome, err := reader.Process(ctx, raw)
		require.NoError(t, err)
		require.Equal(t, "handled", outcome)
	}
	outcome, err := reader.Process(ctx, []byte(`not-json`))
	require.NoError(t, err)
	require.Equal(t, "malformed", outcome)

	// запись старше часа: при первом включении уведомлений история потока ошибок в чат не уходит
	old, err := json.Marshal(platformkafka.ErrorEnvelope{
		Consumer: "analytics", SourceTopic: "test.stock.v1", EventID: uuid.New(),
		EventType: "StockWrittenOff", Error: "old failure", FailedAt: now.Add(-61 * time.Minute),
	})
	require.NoError(t, err)
	outcome, err = reader.Process(ctx, old)
	require.NoError(t, err)
	require.Equal(t, "stale", outcome)

	var count int
	var body string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), max(body) FROM notification_deliveries`).Scan(&count, &body))
	require.Equal(t, 1, count)
	require.Contains(t, body, "Обработчик: analytics")
	require.NotContains(t, body, "secret")
}
