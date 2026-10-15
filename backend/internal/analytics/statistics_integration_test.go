//go:build integration

package analytics_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/analytics"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
)

// companyID - компания, в которой тесты пакета создают каталог.
var companyID = uuid.Must(uuid.NewV7())

// point - точка, на которой тесты пакета ведут склад; её компания - companyID.
var point = organization.Point{ID: uuid.Must(uuid.NewV7()), CompanyID: companyID, Active: true}

func TestStatisticsUseHalfOpenPeriodAndClipIntervals(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	from := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	clk := clock.NewFake(from)
	runner := command.NewRunner(pool, clk, slog.New(slog.DiscardHandler))
	products := catalog.NewService(runner, pool)
	actor := eventstore.Actor{Type: eventstore.ActorUser, ID: "manager"}
	cream, err := products.CreateProduct(ctx, actor, "cream", companyID, catalog.NewProduct{Name: "Сливки", Unit: catalog.UnitMilliliter})
	require.NoError(t, err)
	bacon, err := products.CreateProduct(ctx, actor, "bacon", companyID, catalog.NewProduct{Name: "Бекон", Unit: catalog.UnitGram})
	require.NoError(t, err)
	stock := inventory.NewService(runner, pool, products)
	service := analytics.NewService(pool, products, stock, runner.Now)
	to := from.Add(time.Hour)

	insertWriteOff(t, pool, cream.ID, inventory.WriteOffExpired, 10, from)
	insertWriteOff(t, pool, cream.ID, inventory.WriteOffExpired, 15, from.Add(30*time.Minute))
	insertWriteOff(t, pool, bacon.ID, inventory.WriteOffStorageViolation, 8, from.Add(10*time.Minute))
	insertWriteOff(t, pool, cream.ID, inventory.WriteOffExpired, 100, to)

	writeOffs, err := service.WriteOffs(ctx, point.ID, from, to)
	require.NoError(t, err)
	require.Equal(t, []analytics.WriteOffStatistic{
		// порядок - внутри единицы измерения: граммы и миллилитры между собой не сравниваются
		{Product: bacon, Reason: inventory.WriteOffStorageViolation, Quantity: 8, Operations: 1},
		{Product: cream, Reason: inventory.WriteOffExpired, Quantity: 25, Operations: 2},
	}, writeOffs)

	insertShortage(t, pool, cream.ID, from)
	insertShortage(t, pool, cream.ID, from.Add(20*time.Minute))
	insertShortage(t, pool, bacon.ID, from.Add(5*time.Minute))
	insertInterval(t, pool, cream.ID, from.Add(-20*time.Minute), timePtr(from.Add(10*time.Minute)))
	insertInterval(t, pool, cream.ID, from.Add(20*time.Minute), timePtr(from.Add(30*time.Minute)))
	insertInterval(t, pool, bacon.ID, from.Add(5*time.Minute), nil)

	clk.Advance(2 * time.Hour) // период целиком в прошлом: незакрытый интервал обрезается его концом
	shortages, err := service.Shortages(ctx, point.ID, from, to)
	require.NoError(t, err)
	// порядок - по времени в стоп-листе: у сливок отказов больше, но бекон держал блюдо дольше
	require.Equal(t, []analytics.ShortageStatistic{
		{Product: bacon, RejectedOrders: 1, StopEntries: 1, StopSeconds: 3300},
		{Product: cream, RejectedOrders: 2, StopEntries: 1, StopSeconds: 1200},
	}, shortages)

	// конец периода в будущем: незакрытый интервал длится до текущего момента (два часа от начала),
	// а не до конца месяца
	future, err := service.Shortages(ctx, point.ID, from, from.Add(30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, bacon, future[0].Product)
	require.EqualValues(t, 115*60, future[0].StopSeconds, "from the fifth minute until now")

	empty, err := service.WriteOffs(ctx, point.ID, to.Add(time.Hour), to.Add(2*time.Hour))
	require.NoError(t, err)
	require.Empty(t, empty)
	_, err = service.Shortages(ctx, point.ID, to, from)
	var period *analytics.InvalidPeriodError
	require.ErrorAs(t, err, &period)
}

func insertWriteOff(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, reason inventory.WriteOffReason, quantity int64, at time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO analytics_write_offs
		(event_id, point_id, product_id, reason, quantity, occurred_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), point.ID, productID, reason, quantity, at)
	require.NoError(t, err)
}

func insertShortage(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, at time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO analytics_order_shortages
		(event_id, point_id, product_id, occurred_at) VALUES ($1, $2, $3, $4)`, uuid.New(), point.ID, productID, at)
	require.NoError(t, err)
}

func insertInterval(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, started time.Time, ended *time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO analytics_stop_intervals
		(point_id, dish_id, product_id, started_at, ended_at) VALUES ($1, $2, $3, $4, $5)`, point.ID, uuid.New(), productID, started, ended)
	require.NoError(t, err)
}

func timePtr(value time.Time) *time.Time { return &value }
