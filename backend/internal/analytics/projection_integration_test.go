//go:build integration

package analytics_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/analytics"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/eventstore/estest"
	"goden/backend/internal/platform/postgres/pgtest"
	"goden/backend/internal/platform/projection"
)

func TestProjectionTracksFactsIntervalsAndRebuilds(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	start := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	engine := projection.NewEngine(pool, clock.NewFake(start), slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	productID, dishID := uuid.New(), uuid.New()
	firstBatch, secondBatch := uuid.New(), uuid.New()
	actor := eventstore.Actor{Type: eventstore.ActorUser, ID: "manager"}

	estest.AppendAt(t, pool, organization.PointStream(point.ID), start,
		organization.PointCreated{PointID: point.ID, CompanyID: companyID, Name: "Точка"})
	estest.AppendAt(t, pool, catalog.ProductStream(productID), start,
		catalog.ProductCreated{ProductID: productID, Name: "Сливки", Unit: catalog.UnitMilliliter})
	estest.AppendAt(t, pool, catalog.DishStream(dishID), start,
		catalog.DishCreated{DishID: dishID, CompanyID: companyID, Name: "Паста"},
		catalog.RecipeChanged{DishID: dishID, Ingredients: []catalog.RecipeIngredient{{ProductID: productID, Quantity: 10}}})
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), start.Add(time.Hour),
		inventory.BatchReceived{
			PointID: point.ID, BatchID: firstBatch, ProductID: productID, Quantity: 20,
			ExpiresAt: eventTime(start.Add(4 * time.Hour)), ReceivedAt: eventTime(start.Add(time.Hour)), Source: inventory.BatchFromApproval,
		})
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), start.Add(2*time.Hour),
		inventory.StockConsumed{
			PointID: point.ID, ProductID: productID, OrderID: uuid.New(),
			Batches: []inventory.BatchQuantity{{BatchID: firstBatch, Quantity: 15}},
		})
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), start.Add(3*time.Hour),
		inventory.BatchReceived{
			PointID: point.ID, BatchID: secondBatch, ProductID: productID, Quantity: 20,
			ExpiresAt: eventTime(start.Add(5 * time.Hour)), ReceivedAt: eventTime(start.Add(3 * time.Hour)), Source: inventory.BatchFromApproval,
		})
	estest.AppendAt(t, pool, "order-"+uuid.NewString(), start.Add(6*time.Hour),
		orders.OrderRejected{PointID: point.ID, OrderID: uuid.New(), RejectedBy: actor, Shortages: []orders.Shortage{{ProductID: productID}}})
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), start.Add(6*time.Hour),
		inventory.StockWrittenOff{
			PointID: point.ID, ProductID: productID, BatchID: firstBatch, Quantity: 5,
			Reason: inventory.WriteOffExpired, WrittenOffBy: actor,
		})

	p := analytics.Projection()
	require.NoError(t, engine.CatchUp(ctx, p))
	want := readProjectionState(t, pool)
	require.Equal(t, projectionState{
		writeOffs: 1, shortages: 1, consumed: 15,
		intervals: []stopInterval{
			{started: start, ended: ptr(start.Add(time.Hour))},
			{started: start.Add(2 * time.Hour), ended: ptr(start.Add(3 * time.Hour))},
			{started: start.Add(5 * time.Hour)},
		},
	}, want)

	// Повторный catch-up ничего не применяет повторно.
	require.NoError(t, engine.CatchUp(ctx, p))
	require.Equal(t, want, readProjectionState(t, pool))

	// Новая версия очищает read-модель и получает тот же результат полным replay.
	p.Version++
	require.NoError(t, engine.CatchUp(ctx, p))
	require.Equal(t, want, readProjectionState(t, pool))
}

type stopInterval struct {
	started time.Time
	ended   *time.Time
}

type projectionState struct {
	writeOffs int64
	shortages int64
	consumed  int64
	intervals []stopInterval
}

// Остаток закончился, пополнен и закончился снова, все события с одним временем: интервал
// стоп-листа открывается, закрывается и открывается заново с тем же временем начала. Раньше
// повторное открытие падало на первичном ключе, и проекция останавливалась навсегда.
func TestProjectionReopensStopIntervalAtTheSameMoment(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	at := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	engine := projection.NewEngine(pool, clock.NewFake(at), slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	productID, dishID := uuid.New(), uuid.New()
	first, second := uuid.New(), uuid.New()
	expires := eventTime(at.Add(48 * time.Hour))

	estest.AppendAt(t, pool, organization.PointStream(point.ID), at,
		organization.PointCreated{PointID: point.ID, CompanyID: companyID, Name: "Точка"})
	estest.AppendAt(t, pool, catalog.ProductStream(productID), at,
		catalog.ProductCreated{ProductID: productID, Name: "Сливки", Unit: catalog.UnitMilliliter})
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), at,
		inventory.BatchReceived{PointID: point.ID, BatchID: first, ProductID: productID, Quantity: 10, ExpiresAt: expires, ReceivedAt: eventTime(at), Source: inventory.BatchFromApproval})
	estest.AppendAt(t, pool, catalog.DishStream(dishID), at,
		catalog.DishCreated{DishID: dishID, CompanyID: companyID, Name: "Паста"},
		catalog.RecipeChanged{DishID: dishID, Ingredients: []catalog.RecipeIngredient{{ProductID: productID, Quantity: 10}}})
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), at,
		// остаток закончился: интервал открыт
		inventory.StockConsumed{PointID: point.ID, ProductID: productID, OrderID: uuid.New(), Batches: []inventory.BatchQuantity{{BatchID: first, Quantity: 10}}},
		// пополнен: интервал закрыт в тот же момент
		inventory.BatchReceived{PointID: point.ID, BatchID: second, ProductID: productID, Quantity: 10, ExpiresAt: expires, ReceivedAt: eventTime(at), Source: inventory.BatchFromApproval},
		// закончился снова: интервал открывается с тем же временем начала
		inventory.StockConsumed{PointID: point.ID, ProductID: productID, OrderID: uuid.New(), Batches: []inventory.BatchQuantity{{BatchID: second, Quantity: 10}}},
	)

	require.NoError(t, engine.CatchUp(ctx, analytics.Projection()))

	state := readProjectionState(t, pool)
	require.Equal(t, []stopInterval{{started: at}}, state.intervals, "one open interval")
	require.EqualValues(t, 20, state.consumed)

	// проекция не остановилась: следующее событие применяется
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), at.Add(time.Hour),
		inventory.BatchReceived{PointID: point.ID, BatchID: uuid.New(), ProductID: productID, Quantity: 10, ExpiresAt: expires, ReceivedAt: eventTime(at.Add(time.Hour)), Source: inventory.BatchFromApproval})
	require.NoError(t, engine.CatchUp(ctx, analytics.Projection()))
	require.Equal(t, []stopInterval{{started: at, ended: ptr(at.Add(time.Hour))}}, readProjectionState(t, pool).intervals)
}

// Отключённая точка не продаёт: её открытые интервалы стоп-листа закрываются в момент отключения
// и не открываются снова, иначе время дефицита в её статистике росло бы бесконечно.
func TestProjectionClosesStopIntervalsOfADisabledPoint(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	at := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	engine := projection.NewEngine(pool, clock.NewFake(at), slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	productID, dishID, other := uuid.New(), uuid.New(), uuid.New()

	estest.AppendAt(t, pool, organization.PointStream(point.ID), at,
		organization.PointCreated{PointID: point.ID, CompanyID: companyID, Name: "Точка"})
	estest.AppendAt(t, pool, organization.PointStream(other), at,
		organization.PointCreated{PointID: other, CompanyID: companyID, Name: "Вторая"})
	estest.AppendAt(t, pool, catalog.DishStream(dishID), at,
		catalog.DishCreated{DishID: dishID, CompanyID: companyID, Name: "Паста"},
		// продукта нет ни на одной точке: блюдо сразу в стоп-листе обеих
		catalog.RecipeChanged{DishID: dishID, Ingredients: []catalog.RecipeIngredient{{ProductID: productID, Quantity: 10}}})
	open := func() (n int) {
		require.NoError(t, engine.CatchUp(ctx, analytics.Projection()))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM analytics_stop_intervals WHERE ended_at IS NULL`).Scan(&n))
		return n
	}
	require.Equal(t, 2, open())

	estest.AppendAt(t, pool, organization.PointStream(point.ID), at.Add(time.Hour), organization.PointDisabled{PointID: point.ID})
	require.Equal(t, 1, open(), "the interval of the disabled point is closed")
	var ended time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ended_at FROM analytics_stop_intervals WHERE point_id = $1`, point.ID).Scan(&ended))
	require.True(t, ended.Equal(at.Add(time.Hour)))

	// событие склада отключённой точки (автосписание) интервал заново не открывает
	estest.AppendAt(t, pool, inventory.StockStream(point.ID, productID), at.Add(2*time.Hour),
		inventory.BatchReceived{PointID: point.ID, BatchID: uuid.New(), ProductID: productID, Quantity: 1, ExpiresAt: eventTime(at.Add(48 * time.Hour)), ReceivedAt: eventTime(at.Add(2 * time.Hour)), Source: inventory.BatchFromApproval})
	require.Equal(t, 1, open())

	estest.AppendAt(t, pool, organization.CompanyStream(companyID), at.Add(3*time.Hour), organization.CompanyDisabled{CompanyID: companyID})
	require.Zero(t, open(), "disabling the company closes the intervals of all its points")
}

func TestProjectionTreatsAnArchivedDishAsNotSold(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	at := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	engine := projection.NewEngine(pool, clock.NewFake(at), slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	productID, dishID := uuid.New(), uuid.New()

	estest.AppendAt(t, pool, organization.PointStream(point.ID), at,
		organization.PointCreated{PointID: point.ID, CompanyID: companyID, Name: "Точка"})
	estest.AppendAt(t, pool, catalog.DishStream(dishID), at,
		catalog.DishCreated{DishID: dishID, CompanyID: companyID, Name: "Паста"},
		catalog.RecipeChanged{DishID: dishID, Ingredients: []catalog.RecipeIngredient{{ProductID: productID, Quantity: 10}}})
	open := func() (n int) {
		require.NoError(t, engine.CatchUp(ctx, analytics.Projection()))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM analytics_stop_intervals WHERE ended_at IS NULL`).Scan(&n))
		return n
	}
	require.Equal(t, 1, open())

	estest.AppendAt(t, pool, catalog.DishStream(dishID), at.Add(time.Hour), catalog.DishArchived{DishID: dishID, CompanyID: companyID})
	require.Zero(t, open(), "an archived dish is not sold and makes no shortage")

	estest.AppendAt(t, pool, catalog.DishStream(dishID), at.Add(2*time.Hour), catalog.DishRestored{DishID: dishID, CompanyID: companyID})
	require.Equal(t, 1, open(), "the restored dish is short of the product again")
}

func readProjectionState(t *testing.T, pool *pgxpool.Pool) projectionState {
	t.Helper()
	var state projectionState
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM analytics_write_offs`).Scan(&state.writeOffs))
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM analytics_order_shortages`).Scan(&state.shortages))
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COALESCE(sum(quantity), 0) FROM analytics_consumption`).Scan(&state.consumed))
	rows, err := pool.Query(context.Background(), `SELECT started_at, ended_at FROM analytics_stop_intervals ORDER BY started_at`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var item stopInterval
		require.NoError(t, rows.Scan(&item.started, &item.ended))
		item.started = item.started.UTC()
		if item.ended != nil {
			ended := item.ended.UTC()
			item.ended = &ended
		}
		state.intervals = append(state.intervals, item)
	}
	require.NoError(t, rows.Err())
	return state
}

func eventTime(value time.Time) eventstore.Time { return eventstore.At(value) }

func ptr(value time.Time) *time.Time { return &value }
