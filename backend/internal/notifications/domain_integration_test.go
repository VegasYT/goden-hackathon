//go:build integration

package notifications_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/inventory"
	"goden/backend/internal/notifications"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

func TestDomainConsumerEnqueuesEachEventOnce(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	productID, batchID, eventID := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO catalog_products (id, company_id, name, name_key, unit, category, shelf_life_days)
		VALUES ($1, gen_random_uuid(), 'Сливки', 'сливки', 'ml', 'Молочные продукты', 5)`, productID)
	require.NoError(t, err)
	pointID := pointWithChat(t, pool, "Точка на Ленина", -100500)

	payload, err := json.Marshal(inventory.BatchExpiryWarningIssued{
		PointID: pointID, ProductID: productID, BatchID: batchID, RemainingQuantity: 400,
		ExpiresAt: eventstore.At(now.Add(time.Hour)), WarningPeriodSeconds: 86400,
	})
	require.NoError(t, err)
	envelope := platformkafka.Envelope{
		EnvelopeVersion: platformkafka.EnvelopeVersion,
		EventID:         eventID, EventType: "BatchExpiryWarningIssued", EventVersion: 1,
		StreamID: "stock-" + productID.String(), StreamVersion: 1,
		Payload: payload, Metadata: json.RawMessage(`{}`), OccurredAt: now,
	}

	handler := notifications.NewDomainHandler(notifications.NewQueue(pool), clock.NewFake(now))
	for range 2 {
		require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
			return handler(ctx, tx, envelope)
		}))
	}

	var count int
	var body string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), max(body) FROM notification_deliveries`).Scan(&count, &body))
	require.Equal(t, 1, count)
	require.Contains(t, body, "Продукт: Сливки")
	require.Contains(t, body, "Точка: Точка на Ленина")
}

// pointWithChat создаёт точку с чатом Telegram; chatID == 0 - точка без чата.
func pointWithChat(t *testing.T, pool *pgxpool.Pool, name string, chatID int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	companyID, pointID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO organization_companies (id, name, owner_id) VALUES ($1, 'Компания', 'owner')`, companyID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organization_points (id, company_id, name, name_key, telegram_chat_id)
		VALUES ($1, $2, $3, lower($3), NULLIF($4::bigint, 0))`, pointID, companyID, name, chatID)
	require.NoError(t, err)
	return pointID
}

// Уведомление уходит в чат точки события. Точка без чата уведомлений не получает:
// в общий чат окружения они не попадают, там оказались бы данные чужих компаний.
func TestDomainNotificationGoesToTheChatOfThePoint(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO catalog_products (id, company_id, name, name_key, unit, category, shelf_life_days)
		VALUES ($1, gen_random_uuid(), 'Сливки', 'сливки', 'ml', 'Молочные продукты', 5)`, productID)
	require.NoError(t, err)
	a, b := pointWithChat(t, pool, "Точка А", -1001), pointWithChat(t, pool, "Точка Б", -1002)
	silent := pointWithChat(t, pool, "Без чата", 0)

	handler := notifications.NewDomainHandler(notifications.NewQueue(pool), clock.NewFake(now))
	events := map[uuid.UUID]uuid.UUID{} // событие - точка
	for _, pointID := range []uuid.UUID{a, b, silent, uuid.New()} {
		payload, err := json.Marshal(inventory.BatchExpiryWarningIssued{
			PointID: pointID, ProductID: productID, BatchID: uuid.New(), RemainingQuantity: 400,
			ExpiresAt: eventstore.At(now.Add(time.Hour)), WarningPeriodSeconds: 86400,
		})
		require.NoError(t, err)
		eventID := uuid.New()
		events[eventID] = pointID
		require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
			return handler(ctx, tx, platformkafka.Envelope{
				EnvelopeVersion: platformkafka.EnvelopeVersion, EventID: eventID, EventType: "BatchExpiryWarningIssued",
				EventVersion: 1, Payload: payload, Metadata: json.RawMessage(`{}`), OccurredAt: now,
			})
		}))
	}

	rows, err := pool.Query(ctx, `SELECT source_key, chat_id, body FROM notification_deliveries`)
	require.NoError(t, err)
	defer rows.Close()
	chats := map[uuid.UUID]int64{}
	for rows.Next() {
		var key, body string
		var chatID int64
		require.NoError(t, rows.Scan(&key, &chatID, &body))
		pointID := events[uuid.MustParse(key)]
		chats[pointID] = chatID
		require.Contains(t, body, "Точка: Точка ")
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[uuid.UUID]int64{a: -1001, b: -1002}, chats, "points without a chat and unknown points get nothing")
}

// Новая группа читает топики с начала: без отсечки при первом включении уведомлений
// в чат уходила вся история. Пропуск не ошибка: событие считается обработанным.
func TestDomainConsumerSkipsOldEventsAndExpiredBatches(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	productID, batchID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO catalog_products (id, company_id, name, name_key, unit, category, shelf_life_days)
		VALUES ($1, gen_random_uuid(), 'Сливки', 'сливки', 'ml', 'Молочные продукты', 5)`, productID)
	require.NoError(t, err)
	pointID := pointWithChat(t, pool, "Точка на Ленина", -100500)
	_, err = pool.Exec(ctx, `
		INSERT INTO inventory_batches (id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, point_id)
		VALUES ($1, $2, $3, $4, 400, 0, $5, $6, gen_random_uuid())`, batchID, productID, uuid.New(), uuid.New(), now.Add(-2*time.Hour), now.Add(-48*time.Hour))
	require.NoError(t, err)

	warning := func(expiresAt time.Time) json.RawMessage {
		raw, err := json.Marshal(inventory.BatchExpiryWarningIssued{
			PointID: pointID, ProductID: productID, BatchID: batchID, RemainingQuantity: 400,
			ExpiresAt: eventstore.At(expiresAt), WarningPeriodSeconds: 86400,
		})
		require.NoError(t, err)
		return raw
	}
	expired, err := json.Marshal(inventory.BatchExpired{
		PointID: pointID, ProductID: productID, BatchID: batchID, RemainingQuantity: 400, ExpiresAt: eventstore.At(now.Add(-time.Hour)),
	})
	require.NoError(t, err)

	manualWriteOff, err := json.Marshal(inventory.StockWrittenOff{
		PointID: pointID, ProductID: productID, BatchID: batchID, Quantity: 100, Reason: inventory.WriteOffExpired, Comment: "вручную",
		WrittenOffBy: eventstore.Actor{Type: eventstore.ActorUser, ID: "manager"},
	})
	require.NoError(t, err)

	tests := []struct {
		name       string
		eventType  string
		payload    json.RawMessage
		occurredAt time.Time
		enqueued   bool
	}{
		{"warning issued long ago", "BatchExpiryWarningIssued", warning(now.Add(time.Hour)), now.Add(-61 * time.Minute), false},
		{"fresh warning about a batch that has expired", "BatchExpiryWarningIssued", warning(now.Add(-time.Minute)), now.Add(-10 * time.Minute), false},
		{"expiry noticed long ago", "BatchExpired", expired, now.Add(-25 * time.Hour), false},
		{"fresh warning", "BatchExpiryWarningIssued", warning(now.Add(time.Hour)), now.Add(-59 * time.Minute), true},
		{"fresh expiry notice", "BatchExpired", expired, now.Add(-time.Minute), true},
		// списание просрочки делает человек, уведомление о нём не нужно
		{"manual write-off of an expired batch", "StockWrittenOff", manualWriteOff, now.Add(-time.Minute), false},
	}
	handler := notifications.NewDomainHandler(notifications.NewQueue(pool), clock.NewFake(now))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventID := uuid.New()
			require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
				return handler(ctx, tx, platformkafka.Envelope{
					EnvelopeVersion: platformkafka.EnvelopeVersion,
					EventID:         eventID, EventType: tt.eventType, EventVersion: 1,
					StreamID: "stock-" + productID.String(), StreamVersion: 1,
					Payload: tt.payload, Metadata: json.RawMessage(`{}`), OccurredAt: tt.occurredAt,
				})
			}), "a skipped event is not an error")

			var exists bool
			require.NoError(t, pool.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM notification_deliveries WHERE source_key = $1)`, eventID.String()).Scan(&exists))
			require.Equal(t, tt.enqueued, exists)
		})
	}
}
