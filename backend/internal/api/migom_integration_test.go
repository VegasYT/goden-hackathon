//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/eventstore"
)

func migomAccount(t *testing.T, s apitest.Server) *oas.Client {
	t.Helper()
	return s.As(t, eventstore.Actor{Type: eventstore.ActorService, ID: "migom-integration"}, "migom.integration")
}

// connectMigom подключает к точке заведение Мигома; outlet 0 отключает Мигом.
func connectMigom(t *testing.T, owner *oas.Client, point *oas.Point, outlet int64) *oas.MigomOnPoint {
	t.Helper()
	req := &oas.MigomOnPoint{}
	if outlet > 0 {
		req.OutletID.SetTo(outlet)
	} else {
		req.OutletID.SetToNull()
	}
	got, err := owner.SetMigomOnPoint(ctx, req, oas.SetMigomOnPointParams{PointId: point.ID})
	require.NoError(t, err)
	return got
}

// Мигом подключает к точке её владелец, указывая заведение. Учётная запись Мигома получает
// на подключённой точке права читать меню и оформлять, читать и отменять заказы, и ничего больше.
func TestMigomIsConnectedPerPoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	k.supplyTo(t, k.a, 100)
	manager, _ := hire(t, s, k.owner, k.a, "manager", oas.RoleManager)
	migom := migomAccount(t, s)
	newOrder := &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}

	read, err := k.owner.GetMigomOnPoint(ctx, oas.GetMigomOnPointParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.True(t, read.OutletID.IsNull(), "a new point starts without Migom")

	// управляющий Мигом не подключает
	req := &oas.MigomOnPoint{}
	req.OutletID.SetTo(7)
	_, err = manager.SetMigomOnPoint(ctx, req, oas.SetMigomOnPointParams{PointId: k.a.ID})
	requireForbidden(t, err, "company.manage")

	// пока Мигом не подключён, прав на точке у него нет
	_, err = migom.GetMenu(ctx, oas.GetMenuParams{PointId: k.a.ID})
	requireForbidden(t, err, "kitchen.view")
	_, err = migom.PlaceOrder(ctx, newOrder, oas.PlaceOrderParams{PointId: k.a.ID})
	requireForbidden(t, err, "orders.accept")
	points, err := migom.ListMigomPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, points)

	connected := connectMigom(t, k.owner, k.a, 7)
	require.Equal(t, int64(7), connected.OutletID.Value)
	read, err = k.owner.GetMigomOnPoint(ctx, oas.GetMigomOnPointParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Equal(t, connected, read)
	points, err = migom.ListMigomPoints(ctx)
	require.NoError(t, err)
	require.Equal(t, []oas.MigomPoint{{PointID: k.a.ID, OutletID: 7}}, points)

	menu, err := migom.GetMenu(ctx, oas.GetMenuParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.NotEmpty(t, menu.Items)
	order, err := migom.PlaceOrder(ctx, newOrder, oas.PlaceOrderParams{PointId: k.a.ID})
	require.NoError(t, err)
	_, err = migom.GetOrder(ctx, oas.GetOrderParams{PointId: k.a.ID, OrderId: order.ID})
	require.NoError(t, err)
	// горячие предложения Мигом показывает покупателям
	_, err = migom.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: k.a.ID})
	require.NoError(t, err)
	_, err = migom.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: k.b.ID})
	requireForbidden(t, err, "recommendations.view")

	// лишних прав у Мигома нет, на соседней точке он не подключён
	_, err = migom.GetStock(ctx, oas.GetStockParams{PointId: k.a.ID})
	requireForbidden(t, err, "inventory.view")
	_, err = migom.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "M-1", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 50, oct(15))},
	}, oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireForbidden(t, err, "deliveries.register")
	_, err = migom.SetMigomOnPoint(ctx, req, oas.SetMigomOnPointParams{PointId: k.a.ID})
	requireForbidden(t, err, "company.manage")
	_, err = migom.ListDishes(ctx, oas.ListDishesParams{CompanyId: k.company.ID})
	requireForbidden(t, err, "catalog.view")
	_, err = migom.GetMenu(ctx, oas.GetMenuParams{PointId: k.b.ID})
	requireForbidden(t, err, "kitchen.view")

	// список точек Мигома не виден пользователям, даже владельцу
	_, err = k.owner.ListMigomPoints(ctx)
	requireForbidden(t, err, "migom.integration")

	// отключение отзывает права сразу
	disconnected := connectMigom(t, k.owner, k.a, 0)
	require.True(t, disconnected.OutletID.IsNull())
	_, err = migom.GetMenu(ctx, oas.GetMenuParams{PointId: k.a.ID})
	requireForbidden(t, err, "kitchen.view")
	points, err = migom.ListMigomPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, points)

	t.Run("invalid outlet", func(t *testing.T) {
		resp, data := raw(t, s, http.MethodPut, pointPath(s)+"/migom", `{"outlet_id":0}`, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
		resp, data = raw(t, s, http.MethodPut, pointPath(s)+"/migom", `{}`, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
}

// Учётная запись Мигома одна на все его заведения, поэтому отменить она может только заказы,
// которые оформила сама: заказ официанта и его продукты ей недоступны.
func TestMigomCancelsOnlyItsOwnOrders(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	k.supplyTo(t, k.a, 100)
	waiter, _ := hire(t, s, k.owner, k.a, "waiter", oas.RoleWaiter)
	migom := migomAccount(t, s)
	connectMigom(t, k.owner, k.a, 7)
	newOrder := &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}

	byWaiter, err := waiter.PlaceOrder(ctx, newOrder, oas.PlaceOrderParams{PointId: k.a.ID})
	require.NoError(t, err)
	_, err = migom.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: k.a.ID, OrderId: byWaiter.ID})
	require.Equal(t, "forbidden", requireProblem(t, err, http.StatusForbidden).Type)
	read, err := k.owner.GetOrder(ctx, oas.GetOrderParams{PointId: k.a.ID, OrderId: byWaiter.ID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, read.Status)

	byMigom, err := migom.PlaceOrder(ctx, newOrder, oas.PlaceOrderParams{PointId: k.a.ID})
	require.NoError(t, err)
	cancelled, err := migom.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: k.a.ID, OrderId: byMigom.ID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusCancelled, cancelled.Status)

	// пользователь с правом отмены отменяет любой заказ точки
	_, err = waiter.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: k.a.ID, OrderId: byWaiter.ID})
	require.NoError(t, err)
}

// На отключённой точке Мигом не работает. Отключить его там можно, подключить - нет.
func TestMigomOnADisabledPoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	migom := migomAccount(t, s)
	connectMigom(t, k.owner, k.a, 7)
	connectMigom(t, k.owner, k.b, 8)
	_, err := k.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: k.a.ID})
	require.NoError(t, err)

	_, err = migom.GetMenu(ctx, oas.GetMenuParams{PointId: k.a.ID})
	requireForbidden(t, err, "kitchen.view")
	points, err := migom.ListMigomPoints(ctx)
	require.NoError(t, err)
	require.Equal(t, []oas.MigomPoint{{PointID: k.b.ID, OutletID: 8}}, points)

	require.True(t, connectMigom(t, k.owner, k.a, 0).OutletID.IsNull())
	req := &oas.MigomOnPoint{}
	req.OutletID.SetTo(7)
	_, err = k.owner.SetMigomOnPoint(ctx, req, oas.SetMigomOnPointParams{PointId: k.a.ID})
	require.Equal(t, "conflict", requireProblem(t, err, http.StatusConflict).Type)
}
