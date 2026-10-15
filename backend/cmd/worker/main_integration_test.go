//go:build integration

package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/observability/metricstest"
)

// Задачи worker выполняют проекции: записи появляются в списках без участия теста.
func TestWorkerRunsProjections(t *testing.T) {
	ctx := context.Background()
	s := apitest.New(t)
	background, registry, closeNotifications, err := tasks(s.Pool, s.Clock, slog.New(slog.DiscardHandler),
		appConfig{ExpiryWarningPeriod: 72 * time.Hour, ExpiryCheckInterval: 20 * time.Millisecond}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(closeNotifications)

	runCtx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	for _, run := range background {
		group.Go(func() error { return run(runCtx) })
	}
	t.Cleanup(func() {
		cancel()
		require.NoError(t, group.Wait(), "tasks stop without an error when the context is cancelled")
	})

	product, err := s.Client.CreateProduct(ctx, &oas.NewProduct{Name: "Сливки", Unit: oas.UnitMl}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	dish, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: "Соус"}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	_, err = s.Client.ChangeRecipe(ctx, &oas.Recipe{Ingredients: []oas.Ingredient{{ProductID: product.ID, Quantity: 10}}},
		oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	delivery, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "N", DeliveredAt: s.Clock.Now(),
		Items: []oas.NewDeliveryItem{{ProductID: product.ID, DeclaredQuantity: 100, ExpiresAt: s.Clock.Now().Add(24 * time.Hour)}},
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)
	order, err := s.Client.PlaceOrder(ctx, &oas.NewOrder{Items: []oas.OrderItem{{DishID: dish.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		deliveries, err := s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID})
		require.NoError(t, err)
		orders, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
		require.NoError(t, err)
		return len(deliveries.Items) == 1 && deliveries.Items[0].ID == delivery.ID &&
			len(orders.Items) == 1 && orders.Items[0].ID == order.ID
	}, 20*time.Second, 20*time.Millisecond)

	for _, name := range []string{"inventory-deliveries", "orders-history"} {
		require.Positive(t, metricstest.Value(t, registry, "goden_projection_events_total", "projection", name), name)
		require.Eventually(t, func() bool {
			return metricstest.Value(t, registry, "goden_projection_lag_events", "projection", name) == 0
		}, 10*time.Second, 20*time.Millisecond, name)
	}

	// партия на складе появляется после приёмки
	_, err = s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{
		{ItemID: delivery.Items[0].ID, AcceptedQuantity: 100},
	}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: delivery.ID})
	require.NoError(t, err)

	// контроль сроков: срок партии истёк - worker отмечает просрочку, а списывает человек
	s.Clock.Advance(48 * time.Hour)
	require.Eventually(t, func() bool {
		return metricstest.Value(t, registry, "goden_events_appended_total", "type", "BatchExpired") == 1
	}, 20*time.Second, 20*time.Millisecond, "the expired batch is marked without the test")
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: product.ID})
	require.NoError(t, err)
	require.Len(t, batches.Items, 1)
	require.EqualValues(t, 100, batches.Items[0].Quantity, "the worker does not write off")
	require.Eventually(t, func() bool {
		return metricstest.Value(t, registry, "goden_batches_expired") == 1
	}, 20*time.Second, 20*time.Millisecond)
	require.Positive(t, metricstest.Value(t, registry, "goden_expiry_checks_total", "outcome", "ok"))
	require.Zero(t, metricstest.Value(t, registry, "goden_events_appended_total", "type", "StockWrittenOff"))
}
