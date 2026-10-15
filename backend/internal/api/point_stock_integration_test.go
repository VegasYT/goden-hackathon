//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

// kitchen - компания с двумя точками, продуктом и блюдом из 10 единиц этого продукта.
type kitchen struct {
	network
	cream *oas.Product
	pasta *oas.Dish
}

func newKitchen(t *testing.T, s apitest.Server) kitchen {
	t.Helper()
	k := kitchen{network: newNetwork(t, s)}
	k.cream, k.pasta = ownProduct(t, k.network, "Сливки"), ownDish(t, k.network, "Паста")
	_, err := k.owner.ChangeRecipe(ctx, &oas.Recipe{Ingredients: []oas.Ingredient{{ProductID: k.cream.ID, Quantity: 10}}},
		oas.ChangeRecipeParams{CompanyId: k.company.ID, DishId: k.pasta.ID})
	require.NoError(t, err)
	return k
}

// supplyTo принимает на точку поставку сливок целиком и возвращает поставку.
func (k kitchen) supplyTo(t *testing.T, point *oas.Point, quantity int64) *oas.Delivery {
	t.Helper()
	registered, err := k.owner.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-1", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, quantity, oct(15))},
	}, oas.RegisterDeliveryParams{PointId: point.ID})
	require.NoError(t, err)
	approved, err := k.owner.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: point.ID, DeliveryId: registered.ID})
	require.NoError(t, err)
	return approved
}

func (k kitchen) available(t *testing.T, point *oas.Point) int64 {
	t.Helper()
	stock, err := k.owner.GetStock(ctx, oas.GetStockParams{PointId: point.ID})
	require.NoError(t, err)
	require.Len(t, stock.Items, 1, "the stock lists the products of the company of the point")
	return stock.Items[0].AvailableQuantity
}

func (k kitchen) order(t *testing.T, point *oas.Point, portions int64) *oas.Order {
	t.Helper()
	order, err := k.owner.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: portions}}},
		oas.PlaceOrderParams{PointId: point.ID})
	require.NoError(t, err)
	return order
}

// У каждой точки свой склад: один продукт ведётся на двух точках независимо.
func TestStockIsSeparatePerPoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	onA := k.supplyTo(t, k.a, 100)
	k.supplyTo(t, k.b, 30)
	require.Equal(t, k.a.ID, onA.PointID)
	require.EqualValues(t, 100, k.available(t, k.a))
	require.EqualValues(t, 30, k.available(t, k.b))

	// заказ расходует партии своей точки
	accepted := k.order(t, k.a, 4)
	require.Equal(t, oas.OrderStatusAccepted, accepted.Status)
	require.Equal(t, k.a.ID, accepted.PointID)
	require.EqualValues(t, 60, k.available(t, k.a))
	require.EqualValues(t, 30, k.available(t, k.b))

	// нехватка считается по остатку своей точки, хотя на соседней продукта достаточно
	rejected := k.order(t, k.b, 4)
	require.Equal(t, oas.OrderStatusRejected, rejected.Status)
	require.EqualValues(t, 30, rejected.Shortages[0].AvailableQuantity)
	require.EqualValues(t, 60, k.available(t, k.a))

	// списание и движения - тоже в границах точки
	batchA := onA.Items[0].BatchIds[0]
	_, err := k.owner.WriteOffStock(ctx, &oas.WriteOff{Quantity: 20, Reason: oas.WriteOffReasonOther, Comment: "бой"},
		oas.WriteOffStockParams{PointId: k.a.ID, BatchId: batchA})
	require.NoError(t, err)
	require.EqualValues(t, 40, k.available(t, k.a))
	require.EqualValues(t, 30, k.available(t, k.b))

	movementsA, err := k.owner.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: k.a.ID, ProductId: k.cream.ID})
	require.NoError(t, err)
	movementsB, err := k.owner.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: k.b.ID, ProductId: k.cream.ID})
	require.NoError(t, err)
	require.EqualValues(t, 3, movementsA.Total, "receipt, consumption and write-off of point A")
	require.EqualValues(t, 1, movementsB.Total)

	batchesB, err := k.owner.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: k.b.ID, ProductId: k.cream.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, batchesB.Total)

	// списки поставок и заказов точки
	s.Sync(t)
	deliveries, err := k.owner.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, deliveries.Total)
	require.Equal(t, onA.ID, deliveries.Items[0].ID)
	orders, err := k.owner.ListOrders(ctx, oas.ListOrdersParams{PointId: k.b.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, orders.Total)
	require.Equal(t, rejected.ID, orders.Items[0].ID)

	// статистика списаний ведётся по точкам
	period := func(point *oas.Point) oas.GetWriteOffStatisticsParams {
		return oas.GetWriteOffStatisticsParams{PointId: point.ID, From: oas.NewOptDateTime(oct(1)), To: oas.NewOptDateTime(oct(20))}
	}
	writeOffsA, err := k.owner.GetWriteOffStatistics(ctx, period(k.a))
	require.NoError(t, err)
	require.Len(t, writeOffsA.Items, 1)
	require.EqualValues(t, 20, writeOffsA.Items[0].Quantity)
	writeOffsB, err := k.owner.GetWriteOffStatistics(ctx, period(k.b))
	require.NoError(t, err)
	require.Empty(t, writeOffsB.Items)

	// дефицит: отказ был только на точке Б
	shortage := func(point *oas.Point) []oas.ShortageStatistic {
		stats, err := k.owner.GetShortageStatistics(ctx, oas.GetShortageStatisticsParams{PointId: point.ID, From: oas.NewOptDateTime(oct(1)), To: oas.NewOptDateTime(oct(20))})
		require.NoError(t, err)
		return stats.Items
	}
	require.Len(t, shortage(k.b), 1)
	require.EqualValues(t, 1, shortage(k.b)[0].RejectedOrders)
	for _, item := range shortage(k.a) {
		require.Zero(t, item.RejectedOrders)
	}
}

// Заказы разных точек на одно блюдо оформляются одновременно и не мешают друг другу:
// блокировки берутся по складскому потоку точки.
func TestOrdersOfDifferentPointsRunConcurrently(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	k.supplyTo(t, k.a, 100)
	k.supplyTo(t, k.b, 100)

	const perPoint = 5
	var g errgroup.Group
	for _, point := range []*oas.Point{k.a, k.b} {
		for range perPoint {
			g.Go(func() error {
				order, err := k.owner.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 2}}},
					oas.PlaceOrderParams{PointId: point.ID})
				if err == nil && order.Status != oas.OrderStatusAccepted {
					err = fmt.Errorf("order on point %s is %s", point.ID, order.Status)
				}
				return err
			})
		}
	}
	require.NoError(t, g.Wait())
	require.EqualValues(t, 0, k.available(t, k.a))
	require.EqualValues(t, 0, k.available(t, k.b))
}

// Объект по идентификатору принадлежит точке из адреса: через свою точку нельзя прочитать
// или изменить поставку, партию и заказ другой точки, даже если права есть на обеих.
func TestObjectsOfAnotherPointAreNotFound(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	delivery := k.supplyTo(t, k.a, 100)
	batch := delivery.Items[0].BatchIds[0]
	order := k.order(t, k.a, 1)
	registered, err := k.owner.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-2", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 5, oct(15))},
	}, oas.RegisterDeliveryParams{PointId: k.a.ID})
	require.NoError(t, err)
	before := countEvents(t, s)

	other := k.b.ID
	_, err = k.owner.GetDelivery(ctx, oas.GetDeliveryParams{PointId: other, DeliveryId: delivery.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.GetDeliveryHistory(ctx, oas.GetDeliveryHistoryParams{PointId: other, DeliveryId: delivery.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: other, DeliveryId: registered.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
		Reason: "пересчёт", Items: []oas.ItemCorrection{{ItemID: delivery.Items[0].ID, AcceptedQuantity: 90, RejectedQuantity: 10}},
	}, oas.CorrectDeliveryApprovalParams{PointId: other, DeliveryId: delivery.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.ResolveQuarantine(ctx, &oas.QuarantineResolution{AcceptedQuantity: 1},
		oas.ResolveQuarantineParams{PointId: other, DeliveryId: delivery.ID, ItemId: delivery.Items[0].ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.GetBatch(ctx, oas.GetBatchParams{PointId: other, BatchId: batch})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.WriteOffStock(ctx, &oas.WriteOff{Quantity: 1, Reason: oas.WriteOffReasonOther, Comment: "c"},
		oas.WriteOffStockParams{PointId: other, BatchId: batch})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.AdjustStock(ctx, &oas.Adjustment{Delta: -1, Reason: "пересчёт"}, oas.AdjustStockParams{PointId: other, BatchId: batch})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.GetOrder(ctx, oas.GetOrderParams{PointId: other, OrderId: order.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: other, OrderId: order.ID})
	requireProblem(t, err, http.StatusNotFound)
	require.Equal(t, before, countEvents(t, s), "nothing changed")

	// через свою точку всё доступно
	_, err = k.owner.GetBatch(ctx, oas.GetBatchParams{PointId: k.a.ID, BatchId: batch})
	require.NoError(t, err)
	_, err = k.owner.GetOrder(ctx, oas.GetOrderParams{PointId: k.a.ID, OrderId: order.ID})
	require.NoError(t, err)
}

// Каталог точки - каталог её компании: чужие продукты и блюда на точке не существуют.
func TestPointUsesOnlyTheCatalogOfItsCompany(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	k.supplyTo(t, k.a, 100)
	// каталог другой компании: компании пользователя dev
	salt := create(t, s, "Соль")
	soup := createDish(t, s, "Суп")
	before := countEvents(t, s)

	_, err := k.owner.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-3", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(salt, 5, oct(15))},
	}, oas.RegisterDeliveryParams{PointId: k.a.ID})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Contains(t, problem.Detail.Value, salt.ID.String())

	_, err = k.owner.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: soup.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: k.a.ID})
	requireProblem(t, err, http.StatusUnprocessableEntity)
	_, err = k.owner.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: k.a.ID, ProductId: salt.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = k.owner.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: k.a.ID, ProductId: salt.ID})
	requireProblem(t, err, http.StatusNotFound)
	require.Equal(t, before, countEvents(t, s))

	menu, err := k.owner.GetMenu(ctx, oas.GetMenuParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1, "only the dishes of the company")
	require.Equal(t, k.pasta.ID, menu.Items[0].Dish.ID)
}

// Меню, стоп-лист и рекомендации строятся по остаткам точки; выключенное блюдо на точке не продаётся.
func TestMenuFollowsThePoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	k.supplyTo(t, k.a, 100)

	stopped := func(point *oas.Point) []uuid.UUID {
		list, err := k.owner.GetStopList(ctx, oas.GetStopListParams{PointId: point.ID})
		require.NoError(t, err)
		var ids []uuid.UUID
		for _, item := range list.Items {
			ids = append(ids, item.Dish.ID)
		}
		return ids
	}
	portions := func(point *oas.Point) []int64 {
		menu, err := k.owner.GetMenu(ctx, oas.GetMenuParams{PointId: point.ID})
		require.NoError(t, err)
		var out []int64
		for _, item := range menu.Items {
			out = append(out, item.Portions)
		}
		return out
	}
	require.Equal(t, []int64{10}, portions(k.a))
	require.Empty(t, stopped(k.a))
	require.Equal(t, []int64{0}, portions(k.b), "no stock on point B")
	require.Equal(t, []uuid.UUID{k.pasta.ID}, stopped(k.b))

	// поставка рекомендуется точке, где продукта не хватает на порцию
	advised := func(point *oas.Point) int {
		advice, err := k.owner.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{PointId: point.ID})
		require.NoError(t, err)
		return len(advice.Items)
	}
	require.Zero(t, advised(k.a))
	require.Equal(t, 1, advised(k.b))

	// блюдо выключили на точке Б: его нет ни в меню, ни в стоп-листе, заказать нельзя, поставка не нужна
	_, err := k.owner.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: false}, oas.SetDishOnPointParams{PointId: k.b.ID, DishId: k.pasta.ID})
	require.NoError(t, err)
	require.Empty(t, portions(k.b))
	require.Empty(t, stopped(k.b))
	require.Zero(t, advised(k.b))
	require.Equal(t, []int64{10}, portions(k.a), "point A still sells the dish")

	before := countEvents(t, s)
	_, err = k.owner.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: k.b.ID})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Contains(t, problem.Detail.Value, "не продаётся на этой точке")
	require.Equal(t, before, countEvents(t, s), "no order event for a dish that is not sold")

	forecast, err := k.owner.GetStopListForecast(ctx, oas.GetStopListForecastParams{PointId: k.b.ID})
	require.NoError(t, err)
	require.Empty(t, forecast.Items)
}

// Права на складе точки даёт роль на ней; соседняя точка той же компании закрыта.
func TestStockAccessFollowsTheRoleOnThePoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	approver, _ := hire(t, s, k.owner, k.a, "approver", oas.RoleApprover)
	cook, _ := hire(t, s, k.owner, k.a, "cook", oas.RoleCook)
	waiter, _ := hire(t, s, k.owner, k.a, "waiter", oas.RoleWaiter)
	newDelivery := &oas.NewDelivery{Number: "ПН-4", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 50, oct(15))}}

	// приёмщик принимает, но не регистрирует; повар не видит остатки; официант оформляет заказы
	_, err := approver.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireForbidden(t, err, "deliveries.register")
	registered, err := k.owner.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.a.ID})
	require.NoError(t, err)
	_, err = cook.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: k.a.ID, DeliveryId: registered.ID})
	requireForbidden(t, err, "deliveries.approve")
	_, err = approver.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: k.a.ID, DeliveryId: registered.ID})
	require.NoError(t, err)

	_, err = cook.GetStock(ctx, oas.GetStockParams{PointId: k.a.ID})
	requireForbidden(t, err, "inventory.view")
	_, err = cook.GetMenu(ctx, oas.GetMenuParams{PointId: k.a.ID})
	require.NoError(t, err)
	order, err := waiter.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, order.Status)
	_, err = cook.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: k.a.ID})
	requireForbidden(t, err, "orders.accept")

	// на соседней точке у сотрудников точки А прав нет
	_, err = waiter.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: k.b.ID})
	requireForbidden(t, err, "orders.accept")
	_, err = approver.GetStock(ctx, oas.GetStockParams{PointId: k.b.ID})
	requireForbidden(t, err, "inventory.view")

	// прежних адресов без точки больше нет
	resp, body := raw(t, s, http.MethodGet, "/api/v1/stock", "", nil)
	requireRawProblem(t, resp, body, http.StatusNotFound)
}

// На отключённой точке записи отклоняются, а история остаётся доступной владельцу.
func TestDisabledPointKeepsItsHistory(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	delivery := k.supplyTo(t, k.a, 100)
	_, err := k.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: k.a.ID})
	require.NoError(t, err)
	before := countEvents(t, s)

	_, err = k.owner.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-5", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 5, oct(15))},
	}, oas.RegisterDeliveryParams{PointId: k.a.ID})
	require.Equal(t, "conflict", requireProblem(t, err, http.StatusConflict).Type)
	_, err = k.owner.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: k.a.ID})
	requireProblem(t, err, http.StatusConflict)
	_, err = k.owner.WriteOffStock(ctx, &oas.WriteOff{Quantity: 1, Reason: oas.WriteOffReasonOther, Comment: "c"},
		oas.WriteOffStockParams{PointId: k.a.ID, BatchId: delivery.Items[0].BatchIds[0]})
	requireProblem(t, err, http.StatusConflict)
	require.Equal(t, before, countEvents(t, s))

	require.EqualValues(t, 100, k.available(t, k.a))
	read, err := k.owner.GetDelivery(ctx, oas.GetDeliveryParams{PointId: k.a.ID, DeliveryId: delivery.ID})
	require.NoError(t, err)
	require.Equal(t, delivery.ID, read.ID)
}

// Ни одна запись на отключённой точке и в отключённой компании не проходит, включая те,
// что идут по идентификатору поставки или партии, и каталог компании.
func TestEveryWriteIsRejectedWhenDisabled(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	approved := k.supplyTo(t, k.a, 100)
	pending, err := k.owner.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-2", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 5, oct(15))},
	}, oas.RegisterDeliveryParams{PointId: k.a.ID})
	require.NoError(t, err)
	item, batch := approved.Items[0].ID, approved.Items[0].BatchIds[0]
	placed := k.order(t, k.a, 1)

	pointWrites := map[string]func(point *oas.Point) error{
		"register delivery": func(point *oas.Point) error {
			_, err := k.owner.RegisterDelivery(ctx, &oas.NewDelivery{
				Number: "ПН-9", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 5, oct(15))},
			}, oas.RegisterDeliveryParams{PointId: point.ID})
			return err
		},
		"approve": func(point *oas.Point) error {
			_, err := k.owner.ApproveDelivery(ctx, acceptAllItems(pending), oas.ApproveDeliveryParams{PointId: point.ID, DeliveryId: pending.ID})
			return err
		},
		"resolve quarantine": func(point *oas.Point) error {
			_, err := k.owner.ResolveQuarantine(ctx, &oas.QuarantineResolution{AcceptedQuantity: 1},
				oas.ResolveQuarantineParams{PointId: point.ID, DeliveryId: approved.ID, ItemId: item})
			return err
		},
		"correct approval": func(point *oas.Point) error {
			_, err := k.owner.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
				Reason: "пересчёт", Items: []oas.ItemCorrection{{ItemID: item, AcceptedQuantity: 99, RejectedQuantity: 1}},
			}, oas.CorrectDeliveryApprovalParams{PointId: point.ID, DeliveryId: approved.ID})
			return err
		},
		"write off": func(point *oas.Point) error {
			_, err := k.owner.WriteOffStock(ctx, &oas.WriteOff{Quantity: 1, Reason: oas.WriteOffReasonOther, Comment: "c"},
				oas.WriteOffStockParams{PointId: point.ID, BatchId: batch})
			return err
		},
		"adjust": func(point *oas.Point) error {
			_, err := k.owner.AdjustStock(ctx, &oas.Adjustment{Delta: -1, Reason: "пересчёт"}, oas.AdjustStockParams{PointId: point.ID, BatchId: batch})
			return err
		},
		"order": func(point *oas.Point) error {
			_, err := k.owner.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: point.ID})
			return err
		},
		"dish on point": func(point *oas.Point) error {
			_, err := k.owner.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: false}, oas.SetDishOnPointParams{PointId: point.ID, DishId: k.pasta.ID})
			return err
		},
		"cancel order": func(point *oas.Point) error {
			_, err := k.owner.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: point.ID, OrderId: placed.ID})
			return err
		},
	}
	rejected := func(t *testing.T, writes map[string]func(*oas.Point) error, point *oas.Point) {
		t.Helper()
		before := countEvents(t, s)
		for name, write := range writes {
			require.Equal(t, "conflict", requireProblem(t, write(point), http.StatusConflict).Type, name)
		}
		require.Equal(t, before, countEvents(t, s))
	}

	_, err = k.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: k.a.ID})
	require.NoError(t, err)
	rejected(t, pointWrites, k.a)

	// компанию отключили: её работающая точка тоже закрыта для записи, как и каталог
	_, err = k.owner.DisableCompany(ctx, oas.DisableCompanyParams{CompanyId: k.company.ID})
	require.NoError(t, err)
	rejected(t, map[string]func(*oas.Point) error{
		"register delivery": pointWrites["register delivery"],
		"order":             pointWrites["order"],
		"dish on point":     pointWrites["dish on point"],
		"create product": func(*oas.Point) error {
			_, err := k.owner.CreateProduct(ctx, &oas.NewProduct{Name: "Соль", Unit: oas.UnitG}, oas.CreateProductParams{CompanyId: k.company.ID})
			return err
		},
		"create dish": func(*oas.Point) error {
			_, err := k.owner.CreateDish(ctx, &oas.NewDish{Name: "Суп"}, oas.CreateDishParams{CompanyId: k.company.ID})
			return err
		},
		"change recipe": func(*oas.Point) error {
			_, err := k.owner.ChangeRecipe(ctx, &oas.Recipe{Ingredients: []oas.Ingredient{{ProductID: k.cream.ID, Quantity: 20}}},
				oas.ChangeRecipeParams{CompanyId: k.company.ID, DishId: k.pasta.ID})
			return err
		},
	}, k.b)
}

// Ответ на принятый заказ потерялся, а блюдо выключили или точку отключили: повтор с тем же
// ключом возвращает тот же заказ, а не отказ, и ничего не списывает второй раз.
func TestRepeatedOrderIsReturnedAfterTheDishOrPointIsClosed(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	k.supplyTo(t, k.a, 100)
	request := &oas.NewOrder{Items: []oas.OrderItem{{DishID: k.pasta.ID, Portions: 2}}}
	params := oas.PlaceOrderParams{PointId: k.a.ID, IdempotencyKey: oas.NewOptString("order-1")}
	first, err := k.owner.PlaceOrder(ctx, request, params)
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, first.Status)

	_, err = k.owner.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: false}, oas.SetDishOnPointParams{PointId: k.a.ID, DishId: k.pasta.ID})
	require.NoError(t, err)
	before := countEvents(t, s)
	again, err := k.owner.PlaceOrder(ctx, request, params)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	// новый заказ того же блюда по-прежнему отклоняется
	_, err = k.owner.PlaceOrder(ctx, request, oas.PlaceOrderParams{PointId: k.a.ID, IdempotencyKey: oas.NewOptString("order-2")})
	requireProblem(t, err, http.StatusUnprocessableEntity)

	_, err = k.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: k.a.ID})
	require.NoError(t, err)
	before++ // событие отключения точки
	again, err = k.owner.PlaceOrder(ctx, request, params)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	_, err = k.owner.PlaceOrder(ctx, request, oas.PlaceOrderParams{PointId: k.a.ID, IdempotencyKey: oas.NewOptString("order-3")})
	requireProblem(t, err, http.StatusConflict)

	require.Equal(t, before, countEvents(t, s))
	require.EqualValues(t, 80, k.available(t, k.a))
}

// Позиции поставки и заказа несут название: клиенту не нужно читать каталог ради списка.
func TestItemsCarryProductAndDishNames(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	delivery := k.supplyTo(t, k.a, 100)
	require.Equal(t, "Сливки", delivery.Items[0].ProductName.Value)
	require.Equal(t, k.cream.Unit, delivery.Items[0].Unit.Value)
	order := k.order(t, k.a, 1)
	require.Equal(t, "Паста", order.Items[0].DishName.Value)

	s.Sync(t)
	deliveries, err := k.owner.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Equal(t, "Сливки", deliveries.Items[0].Items[0].ProductName.Value)
	orders, err := k.owner.ListOrders(ctx, oas.ListOrdersParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Equal(t, "Паста", orders.Items[0].Items[0].DishName.Value)
	history, err := k.owner.GetDeliveryHistory(ctx, oas.GetDeliveryHistoryParams{PointId: k.a.ID, DeliveryId: delivery.ID})
	require.NoError(t, err)
	require.Equal(t, "Сливки", history.Items[0].Delivery.Items[0].ProductName.Value)
}

// Статистика без границ периода отвечает за последние семь суток, а не ошибкой.
func TestStatisticsDefaultToTheLastWeek(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	delivery := k.supplyTo(t, k.a, 100)
	_, err := k.owner.WriteOffStock(ctx, &oas.WriteOff{Quantity: 7, Reason: oas.WriteOffReasonOther, Comment: "c"},
		oas.WriteOffStockParams{PointId: k.a.ID, BatchId: delivery.Items[0].BatchIds[0]})
	require.NoError(t, err)
	s.Sync(t)
	s.Clock.Advance(time.Minute) // конец периода исключается, а списание записано в текущий момент

	writeOffs, err := k.owner.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Len(t, writeOffs.Items, 1)
	require.EqualValues(t, 7, writeOffs.Items[0].Quantity)
	_, err = k.owner.GetShortageStatistics(ctx, oas.GetShortageStatisticsParams{PointId: k.a.ID})
	require.NoError(t, err)

	// списание раньше недели назад в период по умолчанию не попадает
	s.Clock.Advance(8 * 24 * time.Hour)
	writeOffs, err = k.owner.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Empty(t, writeOffs.Items)
}
