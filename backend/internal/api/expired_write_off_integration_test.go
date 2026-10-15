//go:build integration

package api_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

// Просрочку никто не списывает сам: блюдо уходит в стоп-лист, партия ждёт решения в списке,
// списывает управляющий.
func TestExpiredBatchWaitsForAManager(t *testing.T) {
	s := apitest.New(t)
	manager := s.Staff(t, "manager", oas.RoleManager)
	cream := create(t, s, "Сливки")
	sauce := dishWith(t, s, "Соус", per(cream, 100))
	batchID := supply(t, s, cream, 400, oct(12))

	stopped := func() []uuid.UUID {
		list, err := manager.GetStopList(ctx, oas.GetStopListParams{PointId: s.PointID})
		require.NoError(t, err)
		var ids []uuid.UUID
		for _, item := range list.Items {
			ids = append(ids, item.Dish.ID)
		}
		return ids
	}
	expired := func() []uuid.UUID {
		page, err := manager.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID})
		require.NoError(t, err)
		var ids []uuid.UUID
		for _, item := range page.Items {
			if item.Expired {
				ids = append(ids, item.Batch.ID)
			}
		}
		return ids
	}
	require.Empty(t, stopped())

	s.Clock.Set(oct(12))
	require.Equal(t, []uuid.UUID{sauce.ID}, stopped(), "an expired product cannot go into a dish")
	require.Equal(t, []uuid.UUID{batchID}, expired())
	placed, err := s.Client.PlaceOrder(ctx, order(portions(sauce, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, placed.Status)

	batch, err := manager.WriteOffStock(ctx, &oas.WriteOff{Quantity: 400, Reason: oas.WriteOffReasonExpired, Comment: "просрочка"},
		oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
	require.NoError(t, err)
	require.Zero(t, batch.Quantity)
	require.Empty(t, expired())
	require.Equal(t, []uuid.UUID{sauce.ID}, stopped(), "without stock the dish stays in the stop list")
}
