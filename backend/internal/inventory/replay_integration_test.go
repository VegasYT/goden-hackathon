//go:build integration

package inventory_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

var ctx = context.Background()

// Тесты этого файла ходят через API: так в складские потоки попадают и события заказов,
// которые сервис inventory сам не порождает.

func apiProduct(t *testing.T, s apitest.Server, name string, unit oas.Unit) *oas.Product {
	t.Helper()
	product, err := s.Client.CreateProduct(ctx, &oas.NewProduct{Name: name, Unit: unit}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	return product
}

func apiDish(t *testing.T, s apitest.Server, name string, ingredients ...oas.Ingredient) *oas.Dish {
	t.Helper()
	dish, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: name}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	dish, err = s.Client.ChangeRecipe(ctx, &oas.Recipe{Ingredients: ingredients}, oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	return dish
}

func apiOrder(t *testing.T, s apitest.Server, dish *oas.Dish, portions int64) *oas.Order {
	t.Helper()
	order, err := s.Client.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: dish.ID, Portions: portions}}}, oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	return order
}

// apiSupply принимает поставку одного продукта целиком и возвращает идентификатор партии.
func apiSupply(t *testing.T, s apitest.Server, product *oas.Product, quantity int64, expiresAt time.Time) uuid.UUID {
	t.Helper()
	delivery, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "N", DeliveredAt: s.Clock.Now(),
		Items: []oas.NewDeliveryItem{{ProductID: product.ID, DeclaredQuantity: quantity, ExpiresAt: expiresAt}},
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)
	delivery, err = s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{
		Items: []oas.ItemDecision{{ItemID: delivery.Items[0].ID, AcceptedQuantity: quantity}},
	}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: delivery.ID})
	require.NoError(t, err)
	return delivery.Items[0].BatchIds[0]
}

func apiQuantities(t *testing.T, s apitest.Server, product *oas.Product) []int64 {
	t.Helper()
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: product.ID})
	require.NoError(t, err)
	out := make([]int64, len(batches.Items))
	for i, batch := range batches.Items {
		out[i] = batch.Quantity
	}
	return out
}

func opt(s string) oas.OptString { return oas.NewOptString(s) }

// tableSnapshot возвращает строки таблицы как JSON в порядке ключа: все столбцы, без пропусков.
func tableSnapshot(t *testing.T, q eventstore.Querier, table, orderBy string) string {
	t.Helper()
	rows, err := q.Query(ctx, `SELECT coalesce(json_agg(row_to_json(t) ORDER BY `+orderBy+`)::text, '[]') FROM `+table+` t`)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var snapshot string
	require.NoError(t, rows.Scan(&snapshot))
	return snapshot
}

// requireStockReplays перестраивает таблицу остатков из событий в откатываемой транзакции
// и сравнивает её с той, что накопили команды.
func requireStockReplays(t *testing.T, s apitest.Server) string {
	t.Helper()
	live := tableSnapshot(t, s.Pool, "inventory_batches", "t.id")

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck
	require.NoError(t, inventory.Rebuild(ctx, tx))
	rebuilt := tableSnapshot(t, tx, "inventory_batches", "t.id")

	require.JSONEq(t, live, rebuilt, "the stock table is a function of the events")
	return live
}

func countEventsOfType(t *testing.T, s apitest.Server, eventType string) int {
	t.Helper()
	var n int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE event_type = $1`, eventType).Scan(&n))
	return n
}

func TestRebuildKnowsEveryEventOfTheModule(t *testing.T) {
	var own int
	for _, info := range eventstore.RegisteredTypes() {
		payload, err := eventstore.Decode(info.Type, info.SchemaVersion, []byte(`{}`))
		if err != nil || !inventory.OwnsEvent(payload) {
			continue
		}
		own++
		require.Contains(t, inventory.Replayers, info.Type, "a new event of the module must be added to the rebuild")
	}
	require.Equal(t, len(inventory.Replayers), own, "the rebuild lists no event that does not exist")
}

func TestRebuildOfEmptyStore(t *testing.T) {
	s := apitest.New(t)
	require.Equal(t, "[]", requireStockReplays(t, s))
}

func TestStockTableReplaysFromEvents(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := apiProduct(t, s, "Сливки", oas.UnitMl), apiProduct(t, s, "Бекон", oas.UnitG)
	pasta := apiDish(t, s, "Паста",
		oas.Ingredient{ProductID: cream.ID, Quantity: 100}, oas.Ingredient{ProductID: bacon.ID, Quantity: 50})

	// частичная приёмка: часть отклонена, часть в карантине
	delivery, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-1", DeliveredAt: s.Clock.Now(),
		Items: []oas.NewDeliveryItem{
			{ProductID: cream.ID, DeclaredQuantity: 1000, ExpiresAt: oct(20)},
			{ProductID: bacon.ID, DeclaredQuantity: 500, ExpiresAt: oct(18)},
		},
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)
	creamItem, baconItem := delivery.Items[0].ID, delivery.Items[1].ID
	_, err = s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{
		{
			ItemID: creamItem, AcceptedQuantity: 600, RejectedQuantity: 100, QuarantinedQuantity: 300,
			RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonPhysicalDamage), QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: opt("c"),
		},
		{ItemID: baconItem, AcceptedQuantity: 400, QuarantinedQuantity: 100, QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: opt("c")},
	}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: delivery.ID})
	require.NoError(t, err)

	// оба выхода из карантина
	_, err = s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{
		AcceptedQuantity: 200, RejectedQuantity: 100, RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: opt("c"),
	}, oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: delivery.ID, ItemId: creamItem})
	require.NoError(t, err)
	_, err = s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{
		RejectedQuantity: 100, RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: opt("c"),
	}, oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: delivery.ID, ItemId: baconItem})
	require.NoError(t, err)

	// корректировка приёмки: сливок принято больше (новая партия), бекона меньше (корректировка остатка)
	_, err = s.Client.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
		Reason: "пересчёт",
		Items: []oas.ItemCorrection{
			{ItemID: creamItem, AcceptedQuantity: 850, RejectedQuantity: 150},
			{ItemID: baconItem, AcceptedQuantity: 350, RejectedQuantity: 150},
		},
	}, oas.CorrectDeliveryApprovalParams{PointId: s.PointID, DeliveryId: delivery.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{600, 200, 50}, apiQuantities(t, s, cream))
	require.Equal(t, []int64{350}, apiQuantities(t, s, bacon))

	// отменённый заказ возвращает взятое в партии
	cancelled := apiOrder(t, s, pasta, 1)
	_, err = s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: cancelled.ID})
	require.NoError(t, err)

	// заказы: принятый расходует две партии сливок, отклонённый ничего не меняет
	require.Equal(t, oas.OrderStatusAccepted, apiOrder(t, s, pasta, 7).Status)
	require.Equal(t, oas.OrderStatusRejected, apiOrder(t, s, pasta, 100).Status)

	// списание, корректировка остатка и предупреждение о сроке
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	last := batches.Items[len(batches.Items)-1].ID
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 30, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: last})
	require.NoError(t, err)
	_, err = s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: 5, Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: last})
	require.NoError(t, err)

	// предупреждение о сроке - команда для worker, в API её нет
	runner := command.NewRunner(s.Pool, s.Clock, slog.New(slog.DiscardHandler))
	service := inventory.NewService(runner, s.Pool, catalog.NewService(runner, s.Pool))
	worker := eventstore.Actor{Type: eventstore.ActorService, ID: "worker"}
	issued, err := service.IssueBatchExpiryWarning(ctx, worker, last, 30*24*time.Hour)
	require.NoError(t, err)
	require.True(t, issued)

	// срок истёк: отметка о просрочке, остаток не меняется
	s.Clock.Set(oct(20))
	noticed, err := service.MarkBatchExpired(ctx, worker, last)
	require.NoError(t, err)
	require.True(t, noticed)

	// сценарий содержит каждое событие, которое меняет таблицу остатков
	for eventType, replay := range inventory.Replayers {
		if replay != nil {
			require.Positive(t, countEventsOfType(t, s, eventType), eventType)
		}
	}

	snapshot := requireStockReplays(t, s)
	require.Contains(t, snapshot, last.String())
	require.Equal(t, []int64{0, 100, 25}, apiQuantities(t, s, cream), "700 consumed by FEFO, then 30 written off and 5 added")
	require.Equal(t, []int64{0}, apiQuantities(t, s, bacon))
}

// Каждое событие, меняющее таблицу остатков, - это событие складского потока, и у него
// должен быть вид движения: иначе чтение движений продукта вернёт ошибку.
func TestEveryStockEventIsAMovement(t *testing.T) {
	for eventType, replay := range inventory.Replayers {
		// регистрация поставки меняет только таблицу продуктов из поставок, остатков она не касается
		if replay == nil || eventType == "DeliveryRegistered" {
			continue
		}
		movement, err := inventory.MovementFromEvent(eventstore.Event{
			NewEvent: eventstore.NewEvent{Type: eventType, SchemaVersion: 1, Payload: []byte(`{}`), Metadata: []byte(`{}`)},
		})
		require.NoError(t, err, eventType)
		require.NotEmpty(t, movement.Type, eventType)
	}
}

// Расход отменённого заказа - не спрос: рекомендации поставок считают его так, будто заказа не было.
func TestConsumptionSkipsCancelledOrders(t *testing.T) {
	s := apitest.New(t)
	cream := apiProduct(t, s, "Сливки", oas.UnitMl)
	pasta := apiDish(t, s, "Паста", oas.Ingredient{ProductID: cream.ID, Quantity: 100})
	apiSupply(t, s, cream, 1000, oct(20))
	runner := command.NewRunner(s.Pool, s.Clock, slog.New(slog.DiscardHandler))
	service := inventory.NewService(runner, s.Pool, catalog.NewService(runner, s.Pool))
	start := s.Clock.Now()

	cancelled := apiOrder(t, s, pasta, 3)
	s.Clock.Advance(time.Hour)
	require.Equal(t, oas.OrderStatusAccepted, apiOrder(t, s, pasta, 1).Status)
	_, err := s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: cancelled.ID})
	require.NoError(t, err)

	consumed, first, err := service.Consumption(ctx, s.PointID, start)
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]int64{cream.ID: 100}, consumed, "only the order that was not cancelled")
	require.Equal(t, start.Add(time.Hour).UTC(), first, "the cancelled order does not start the observed period")
}

// Расход - это только списания под заказы за период: порча и корректировки спросом не являются.
func TestConsumptionCountsOnlyOrders(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := apiProduct(t, s, "Сливки", oas.UnitMl), apiProduct(t, s, "Бекон", oas.UnitG)
	pasta := apiDish(t, s, "Паста",
		oas.Ingredient{ProductID: cream.ID, Quantity: 100}, oas.Ingredient{ProductID: bacon.ID, Quantity: 50})
	early := apiSupply(t, s, cream, 150, oct(12))
	apiSupply(t, s, cream, 1000, oct(20))
	apiSupply(t, s, bacon, 1000, oct(20))
	runner := command.NewRunner(s.Pool, s.Clock, slog.New(slog.DiscardHandler))
	service := inventory.NewService(runner, s.Pool, catalog.NewService(runner, s.Pool))
	start := s.Clock.Now()

	require.Equal(t, oas.OrderStatusAccepted, apiOrder(t, s, pasta, 2).Status) // 200 сливок из двух партий
	s.Clock.Advance(3 * time.Hour)
	require.Equal(t, oas.OrderStatusAccepted, apiOrder(t, s, pasta, 1).Status)
	require.Equal(t, oas.OrderStatusRejected, apiOrder(t, s, pasta, 1000).Status)
	_, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 300, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: early})
	require.Error(t, err, "the early batch is already empty")
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 300, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batches.Items[1].ID})
	require.NoError(t, err)
	_, err = s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: -40, Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: batches.Items[1].ID})
	require.NoError(t, err)

	all, first, err := service.Consumption(ctx, s.PointID, start)
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]int64{cream.ID: 300, bacon.ID: 150}, all, "three accepted portions; write-off, adjustment and the rejected order do not count")

	recent, _, err := service.Consumption(ctx, s.PointID, start.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]int64{cream.ID: 100, bacon.ID: 50}, recent, "only the order inside the period")

	none, noFirst, err := service.Consumption(ctx, s.PointID, s.Clock.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Empty(t, none)
	require.True(t, noFirst.IsZero(), "no consumption in the period")
	require.Equal(t, start.UTC(), first, "the earliest consumption in the period")
}
