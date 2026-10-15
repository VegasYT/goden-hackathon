//go:build integration

package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func TestOrderCancellationReturnsStock(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 150, oct(20))
	supply(t, s, cream, 1000, oct(25))

	placed, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 2)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, []int64{0, 950}, quantities(t, s, cream), "FEFO: 150 from the early batch, 50 from the late one")

	params := oas.CancelOrderParams{PointId: s.PointID, OrderId: placed.ID, IdempotencyKey: oas.NewOptString("cancel-1")}
	cancelled, err := s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, params)
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusCancelled, cancelled.Status)
	require.Equal(t, s.Clock.Now(), cancelled.CancelledAt.Value)
	require.Equal(t, []int64{150, 1000}, quantities(t, s, cream), "every batch got back exactly what was taken")

	s.Clock.Advance(time.Minute)
	repeated, err := s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, params)
	require.NoError(t, err)
	require.Equal(t, cancelled, repeated, "the same key returns the same result")
	require.Equal(t, []int64{150, 1000}, quantities(t, s, cream), "the stock is returned once")

	_, err = s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: placed.ID})
	require.Equal(t, "order-not-cancellable", requireProblem(t, err, http.StatusConflict).Type)

	read, err := s.Client.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: placed.ID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusCancelled, read.Status)
	require.Equal(t, cancelled.CancelledAt, read.CancelledAt)

	movements, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	require.Equal(t, oas.MovementTypeReturned, movements.Items[0].Type, "the newest movement is the return")
	require.Equal(t, placed.ID, movements.Items[0].OrderID.Value)
	require.Len(t, movements.Items[0].Changes, 2)

	s.Sync(t)
	history, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID, Status: oas.NewOptListOrdersStatus(oas.ListOrdersStatusCancelled)})
	require.NoError(t, err)
	require.Len(t, history.Items, 1)
	require.Equal(t, placed.ID, history.Items[0].ID)
	require.Equal(t, cancelled.CancelledAt, history.Items[0].CancelledAt)
	accepted, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID, Status: oas.NewOptListOrdersStatus(oas.ListOrdersStatusAccepted)})
	require.NoError(t, err)
	require.Empty(t, accepted.Items)
}

func TestOnlyAnAcceptedOrderIsCancelled(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 500, oct(20))

	rejected, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 10)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, rejected.Status)
	_, err = s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: rejected.ID})
	require.Equal(t, "order-not-cancellable", requireProblem(t, err, http.StatusConflict).Type)

	accepted, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	cook := s.Staff(t, "cook", oas.RoleCook)
	_, err = cook.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: accepted.ID})
	requireProblem(t, err, http.StatusForbidden)
	require.Equal(t, []int64{400}, quantities(t, s, cream), "nothing returned")
}

// Партия истекла между заказом и отменой: возвращённое просрочено и доступным не становится.
func TestReturnToAnExpiredBatchStaysExpired(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 300, s.Clock.Now().Add(2*time.Hour))
	supply(t, s, cream, 1000, oct(25))

	placed, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 2)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	s.Clock.Advance(3 * time.Hour)
	_, err = s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: placed.ID})
	require.NoError(t, err)

	stock, err := s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Len(t, stock.Items, 1)
	require.EqualValues(t, 1000, stock.Items[0].AvailableQuantity, "only the fresh batch is available")
	require.EqualValues(t, 300, stock.Items[0].ExpiredQuantity, "the returned 200 are expired along with the rest of the batch")
}

// Отменённый заказ не ускоряет прогноз стоп-листа: его расход исчезает из аналитики.
func TestCancelledOrderIsNotDemand(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 1000, oct(25))

	placed, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 5)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	s.Clock.Advance(time.Hour)
	s.Sync(t)
	forecast, err := s.Client.GetStopListForecast(ctx, oas.GetStopListForecastParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Len(t, forecast.Items, 1, "500 per hour empties the rest within the horizon")

	_, err = s.Client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: placed.ID})
	require.NoError(t, err)
	s.Sync(t)
	forecast, err = s.Client.GetStopListForecast(ctx, oas.GetStopListForecastParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Empty(t, forecast.Items, "without demand the cream lasts")
}
