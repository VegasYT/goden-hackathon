//go:build integration

package api_test

import (
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/projection"
	"goden/backend/internal/readmodel"
)

func listDeliveries(t *testing.T, s apitest.Server, params oas.ListDeliveriesParams) *oas.DeliveryPage {
	t.Helper()
	s.Sync(t)
	page, err := s.Client.ListDeliveries(ctx, params)
	require.NoError(t, err)
	return page
}

func listOrders(t *testing.T, s apitest.Server, params oas.ListOrdersParams) *oas.OrderPage {
	t.Helper()
	s.Sync(t)
	page, err := s.Client.ListOrders(ctx, params)
	require.NoError(t, err)
	return page
}

func deliveryIDs(page *oas.DeliveryPage) []uuid.UUID {
	ids := make([]uuid.UUID, len(page.Items))
	for i, delivery := range page.Items {
		ids[i] = delivery.ID
	}
	return ids
}

func orderIDs(page *oas.OrderPage) []uuid.UUID {
	ids := make([]uuid.UUID, len(page.Items))
	for i, order := range page.Items {
		ids[i] = order.ID
	}
	return ids
}

// Списки строятся асинхронно: до обработки событий проекцией записи в них нет.
func TestListsFollowTheEventStore(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 10, oct(15)))

	before, err := s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Empty(t, before.Items, "the projection has not processed the event yet")

	// чтение по идентификатору от проекции не зависит
	_, err = s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)

	after := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID})
	require.Equal(t, []oas.Delivery{*registered}, after.Items)
	require.EqualValues(t, 1, after.Total)
}

func TestDeliveryList(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	// пять поставок с разными датами, от ранней к поздней
	var all []*oas.Delivery
	for range 5 {
		all = append(all, registerDelivery(t, s, deliveryItem(cream, 100, oct(20))))
		s.Clock.Advance(time.Hour)
	}
	first, second := all[0], all[1]

	// первая принята целиком, вторая - с карантином
	_, err := s.Client.ApproveDelivery(ctx, acceptAllItems(first), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: first.ID})
	require.NoError(t, err)
	_, err = s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{{
		ItemID: second.Items[0].ID, AcceptedQuantity: 70, QuarantinedQuantity: 30,
		QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("c"),
	}}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: second.ID})
	require.NoError(t, err)

	t.Run("order and pagination", func(t *testing.T) {
		page := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Limit: oas.NewOptInt32(2)})
		require.Equal(t, []uuid.UUID{all[4].ID, all[3].ID}, deliveryIDs(page), "latest delivery date first")
		require.EqualValues(t, 5, page.Total)

		next := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(4)})
		require.Equal(t, []uuid.UUID{first.ID}, deliveryIDs(next))
	})
	t.Run("queue for approval", func(t *testing.T) {
		page := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Status: oas.NewOptListDeliveriesStatus(oas.ListDeliveriesStatusRegistered)})
		require.Equal(t, []uuid.UUID{all[4].ID, all[3].ID, all[2].ID}, deliveryIDs(page))
		require.EqualValues(t, 3, page.Total)

		approved := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Status: oas.NewOptListDeliveriesStatus(oas.ListDeliveriesStatusApproved)})
		require.Equal(t, []uuid.UUID{second.ID, first.ID}, deliveryIDs(approved))
	})

	openQuarantine := oas.ListDeliveriesParams{PointId: s.PointID, Quarantine: oas.NewOptListDeliveriesQuarantine(oas.ListDeliveriesQuarantineOpen)}
	t.Run("open quarantine", func(t *testing.T) {
		page := listDeliveries(t, s, openQuarantine)
		require.Equal(t, []uuid.UUID{second.ID}, deliveryIDs(page))
		require.EqualValues(t, 1, page.Total)
		require.Equal(t, []int64{70, 0, 30}, itemDistribution(page.Items[0].Items[0]))
	})
	t.Run("quarantine resolved", func(t *testing.T) {
		_, err := s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{
			AcceptedQuantity: 20, RejectedQuantity: 10,
			RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("c"),
		}, oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: second.ID, ItemId: second.Items[0].ID})
		require.NoError(t, err)

		require.Empty(t, listDeliveries(t, s, openQuarantine).Items)
		page := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Status: oas.NewOptListDeliveriesStatus(oas.ListDeliveriesStatusApproved)})
		require.Equal(t, []int64{90, 10, 0}, itemDistribution(page.Items[0].Items[0]))
		require.Len(t, page.Items[0].Items[0].BatchIds, 2)
	})
	t.Run("approval corrected", func(t *testing.T) {
		_, err := s.Client.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
			Reason: "пересчёт",
			Items:  []oas.ItemCorrection{{ItemID: second.Items[0].ID, AcceptedQuantity: 95, RejectedQuantity: 5}},
		}, oas.CorrectDeliveryApprovalParams{PointId: s.PointID, DeliveryId: second.ID})
		require.NoError(t, err)

		page := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Status: oas.NewOptListDeliveriesStatus(oas.ListDeliveriesStatusApproved)})
		require.Equal(t, []int64{95, 5, 0}, itemDistribution(page.Items[0].Items[0]))
		require.Len(t, page.Items[0].Items[0].BatchIds, 3)
	})
	t.Run("every element equals the delivery read by id", func(t *testing.T) {
		page := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID})
		require.Len(t, page.Items, 5)
		for _, listed := range page.Items {
			read, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: listed.ID})
			require.NoError(t, err)
			require.Equal(t, *read, listed)
		}
	})
	t.Run("unknown filter value", func(t *testing.T) {
		for _, query := range []string{"?status=lost", "?quarantine=closed", "?limit=0"} {
			resp, body := raw(t, s, http.MethodGet, pointPath(s)+"/deliveries"+query, "", nil)
			requireRawProblem(t, resp, body, http.StatusBadRequest)
		}
	})
}

func TestOrderHistory(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 100)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	supply(t, s, cream, 350, oct(20))

	// три принятых заказа, затем два отклонённых: на четвёртый продукта уже не хватает
	var placed []*oas.Order
	for range 5 {
		s.Clock.Advance(time.Minute)
		placedAt := s.Clock.Now()
		order, err := s.Client.PlaceOrder(ctx, order(portions(dish, 1)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		require.Equal(t, placedAt, order.PlacedAt.Value, "placed at the time of the command")
		placed = append(placed, order)
	}
	require.Equal(t, oas.OrderStatusAccepted, placed[2].Status)
	require.Equal(t, oas.OrderStatusRejected, placed[3].Status)

	t.Run("order and pagination", func(t *testing.T) {
		page := listOrders(t, s, oas.ListOrdersParams{PointId: s.PointID, Limit: oas.NewOptInt32(2)})
		require.Equal(t, []uuid.UUID{placed[4].ID, placed[3].ID}, orderIDs(page), "latest order first")
		require.EqualValues(t, 5, page.Total)
	})
	t.Run("filter by status", func(t *testing.T) {
		rejected := listOrders(t, s, oas.ListOrdersParams{PointId: s.PointID, Status: oas.NewOptListOrdersStatus(oas.ListOrdersStatusRejected)})
		require.Equal(t, []uuid.UUID{placed[4].ID, placed[3].ID}, orderIDs(rejected))
		require.EqualValues(t, 2, rejected.Total)
		require.Equal(t, []oas.Shortage{{ProductID: cream.ID, Product: oas.NewOptProduct(*cream), RequiredQuantity: 100, AvailableQuantity: 50}}, rejected.Items[0].Shortages)

		accepted := listOrders(t, s, oas.ListOrdersParams{PointId: s.PointID, Status: oas.NewOptListOrdersStatus(oas.ListOrdersStatusAccepted)})
		require.EqualValues(t, 3, accepted.Total)
	})
	t.Run("every element equals the order read by id and the command result", func(t *testing.T) {
		page := listOrders(t, s, oas.ListOrdersParams{PointId: s.PointID})
		require.Len(t, page.Items, 5)
		for i, listed := range page.Items {
			read, err := s.Client.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: listed.ID})
			require.NoError(t, err)
			require.Equal(t, *read, listed)
			require.Equal(t, *placed[len(placed)-1-i], listed)
		}
	})
	t.Run("repeated request with the same key", func(t *testing.T) {
		params := oas.PlaceOrderParams{PointId: s.PointID, IdempotencyKey: oas.NewOptString("order-1")}
		first, err := s.Client.PlaceOrder(ctx, order(portions(dish, 1)), params)
		require.NoError(t, err)
		s.Clock.Advance(time.Minute)
		second, err := s.Client.PlaceOrder(ctx, order(portions(dish, 1)), params)
		require.NoError(t, err)
		require.Equal(t, first, second, "the stored result keeps the original time")
		require.EqualValues(t, 6, listOrders(t, s, oas.ListOrdersParams{PointId: s.PointID}).Total)
	})
	t.Run("unknown filter value", func(t *testing.T) {
		resp, body := raw(t, s, http.MethodGet, pointPath(s)+"/orders?status=lost", "", nil)
		requireRawProblem(t, resp, body, http.StatusBadRequest)
	})
}

// Перестроенные с нуля проекции совпадают с накопленными при обычной работе.
func TestListProjectionsRebuild(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 100)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	for range 3 {
		supply(t, s, cream, 150, oct(20))
		_, err := s.Client.PlaceOrder(ctx, order(portions(dish, 2)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		s.Clock.Advance(time.Minute)
	}
	registerDelivery(t, s, deliveryItem(cream, 10, oct(15)))
	s.Sync(t)

	snapshot := func() string {
		var out string
		require.NoError(t, s.Pool.QueryRow(ctx, `
			SELECT (SELECT json_agg(row_to_json(d) ORDER BY d.id) FROM inventory_deliveries d)::text ||
			       (SELECT json_agg(row_to_json(o) ORDER BY o.id) FROM orders_history o)::text`).Scan(&out))
		return out
	}
	original := snapshot()
	require.Contains(t, original, `"status":"registered"`)

	engine := projection.NewEngine(s.Pool, s.Clock, slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	engine.BatchSize = 4 // несколько порций
	for _, p := range readmodel.Projections() {
		p.Version++
		require.NoError(t, engine.CatchUp(ctx, p), p.Name)
	}
	require.Equal(t, original, snapshot())
}
