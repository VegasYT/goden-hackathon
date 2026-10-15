//go:build integration

package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

// Отменять заказы точки могут владелец, управляющий и официант; приёмщик и повар - нет.
func TestOrderCancelRights(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 1000, oct(25))
	place := func() uuid.UUID {
		placed, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		return placed.ID
	}
	cancel := func(client *oas.Client, id uuid.UUID) error {
		_, err := client.CancelOrder(ctx, oas.OptOrderCancellation{}, oas.CancelOrderParams{PointId: s.PointID, OrderId: id})
		return err
	}

	for _, role := range []oas.Role{oas.RoleManager, oas.RoleWaiter} {
		require.NoError(t, cancel(s.Staff(t, string(role), role), place()), role)
	}
	for _, role := range []oas.Role{oas.RoleApprover, oas.RoleCook} {
		requireForbidden(t, cancel(s.Staff(t, string(role), role), place()), "orders.cancel")
	}
}

func TestOrderCancelComment(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 1000, oct(25))
	cancel := func(comment oas.OptOrderCancellation) (*oas.Order, error) {
		placed, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		return s.Client.CancelOrder(ctx, comment, oas.CancelOrderParams{PointId: s.PointID, OrderId: placed.ID})
	}
	with := func(comment string) oas.OptOrderCancellation {
		return oas.NewOptOrderCancellation(oas.OrderCancellation{Comment: oas.NewOptString(comment)})
	}

	bare, err := cancel(oas.OptOrderCancellation{})
	require.NoError(t, err)
	require.False(t, bare.CancelComment.Set, "no body - no comment")

	commented, err := cancel(with("  гость ушёл  "))
	require.NoError(t, err)
	require.Equal(t, "гость ушёл", commented.CancelComment.Value)

	blank, err := cancel(with("   "))
	require.NoError(t, err)
	require.False(t, blank.CancelComment.Set, "a blank comment is not kept")

	_, err = cancel(with(strings.Repeat("я", 1001)))
	requireProblem(t, err, http.StatusBadRequest)

	read, err := s.Client.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: commented.ID})
	require.NoError(t, err)
	require.Equal(t, commented.CancelComment, read.CancelComment)
}

// По заказу видно, кто его оформил и кто отменил: заказ официанта отличается от заказа Мигома,
// а Мигом читает отмену своего заказа сотрудником Годена вместе с комментарием.
func TestOrderAuthors(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки")
	pasta := dishWith(t, s, "Паста", per(cream, 100))
	supply(t, s, cream, 1000, oct(25))
	waiter := s.Staff(t, "waiter", oas.RoleWaiter)
	manager := s.Staff(t, "manager", oas.RoleManager)
	migom := migomAccount(t, s)
	connected := &oas.MigomOnPoint{}
	connected.OutletID.SetTo(7)
	_, err := s.Client.SetMigomOnPoint(ctx, connected, oas.SetMigomOnPointParams{PointId: s.PointID})
	require.NoError(t, err)

	byWaiter, err := waiter.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "waiter", Name: oas.NewOptString("waiter")}, byWaiter.PlacedBy)
	byMigom, err := migom.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeService, ID: "migom-integration"}, byMigom.PlacedBy)

	reason := oas.NewOptOrderCancellation(oas.OrderCancellation{Comment: oas.NewOptString("нет курицы")})
	_, err = manager.CancelOrder(ctx, reason, oas.CancelOrderParams{PointId: s.PointID, OrderId: byMigom.ID})
	require.NoError(t, err)

	seen, err := migom.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: byMigom.ID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusCancelled, seen.Status)
	require.Equal(t, oas.NewOptActor(oas.Actor{Type: oas.ActorTypeUser, ID: "manager", Name: oas.NewOptString("manager")}), seen.CancelledBy)
	require.Equal(t, "нет курицы", seen.CancelComment.Value)
	require.Equal(t, byMigom.PlacedBy, seen.PlacedBy)

	s.Sync(t)
	history, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Len(t, history.Items, 2)
	require.Equal(t, seen, &history.Items[0], "history and reading by id agree")
	require.Equal(t, byWaiter.PlacedBy, history.Items[1].PlacedBy)
	require.False(t, history.Items[1].CancelledBy.Set)

	// строки истории, построенные до появления автора, получают его при перестройке
	_, err = s.Pool.Exec(ctx, `UPDATE orders_history SET state = state - 'placed_by' - 'cancelled_by' - 'cancel_comment'`)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `UPDATE projection_checkpoints SET version = 2 WHERE name = 'orders-history'`)
	require.NoError(t, err)
	s.Sync(t)
	rebuilt, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, history, rebuilt)
}
