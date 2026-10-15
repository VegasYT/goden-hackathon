//go:build integration

package notifications_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/notifications"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

func TestQueueDeduplicatesClaimsAndRecoversExpiredLease(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	queue := notifications.NewQueue(pool)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

	enqueue := func(tx pgx.Tx) error {
		delivery := notifications.Delivery{
			SourceKey: "event-1", Kind: notifications.KindExpiryWarning, Body: "body",
		}
		require.NoError(t, queue.Enqueue(ctx, tx, delivery, now))
		return queue.Enqueue(ctx, tx, delivery, now)
	}
	require.NoError(t, postgres.InTx(ctx, pool, enqueue))

	first, ok, err := queue.Claim(ctx, now, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "event-1", first.SourceKey)
	require.Equal(t, 1, first.Attempts)

	_, ok, err = queue.Claim(ctx, now, time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "active lease must hide the delivery")

	recovered, ok, err := queue.Claim(ctx, now.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2, recovered.Attempts)

	require.NoError(t, queue.Retry(ctx, recovered.SourceKey, now.Add(3*time.Minute), "temporary"))
	_, ok, err = queue.Claim(ctx, now.Add(2*time.Minute), time.Minute)
	require.NoError(t, err)
	require.False(t, ok)

	retried, ok, err := queue.Claim(ctx, now.Add(3*time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 3, retried.Attempts)
	require.NoError(t, queue.MarkSent(ctx, retried.SourceKey, now.Add(3*time.Minute)))

	_, ok, err = queue.Claim(ctx, now.Add(10*time.Minute), time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
	pending, err := queue.Pending(ctx)
	require.NoError(t, err)
	require.Zero(t, pending)

	var total int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notification_deliveries`).Scan(&total))
	require.Equal(t, 1, total)
}

// Очистка удаляет только завершённые старые записи: ожидающие остаются, сколько бы они ни ждали.
func TestQueuePrunesOldFinishedDeliveries(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	queue := notifications.NewQueue(pool)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	old := now.Add(-8 * 24 * time.Hour)
	require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		for key, at := range map[string]time.Time{"old-sent": old, "old-failed": old, "old-pending": old, "fresh-sent": now} {
			if err := queue.Enqueue(ctx, tx, notifications.Delivery{SourceKey: key, Kind: notifications.KindExpiryWarning, Body: key}, at); err != nil {
				return err
			}
		}
		return nil
	}))
	_, err := pool.Exec(ctx, `UPDATE notification_deliveries SET status = 'sent', sent_at = created_at WHERE source_key IN ('old-sent', 'fresh-sent')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE notification_deliveries SET status = 'failed' WHERE source_key = 'old-failed'`)
	require.NoError(t, err)

	removed, err := queue.Prune(ctx, now.Add(-7*24*time.Hour))
	require.NoError(t, err)
	require.EqualValues(t, 2, removed)
	var left []string
	rows, err := pool.Query(ctx, `SELECT source_key FROM notification_deliveries ORDER BY source_key`)
	require.NoError(t, err)
	left, err = pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	require.Equal(t, []string{"fresh-sent", "old-pending"}, left)
}
