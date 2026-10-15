package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goden/backend/internal/orders/db"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/projection"
)

// HistoryProjection ведёт таблицу orders_history для списка заказов: строка появляется
// с принятием или отказом и меняется при отмене.
func HistoryProjection() projection.Projection {
	return projection.Projection{
		// версия 3: у заказа есть автор оформления и отмены, у старых строк его не было
		Name: "orders-history", Version: 3,
		Apply: func(ctx context.Context, tx pgx.Tx, e eventstore.Event) error {
			if !orderEvents[e.Type] {
				return nil
			}
			q := db.New(tx)
			var order Order
			if e.Type == "OrderCancelled" {
				state, err := q.OrderState(ctx, orderIDOf(e))
				if err != nil {
					return fmt.Errorf("orders: read history of cancelled order: %w", err)
				}
				if err := json.Unmarshal(state, &order); err != nil {
					return fmt.Errorf("orders: decode order state: %w", err)
				}
			}
			if err := applyOrderEvent(&order, e); err != nil {
				return err
			}
			state, err := json.Marshal(order)
			if err != nil {
				return fmt.Errorf("orders: encode order %s: %w", order.ID, err)
			}
			if e.Type == "OrderCancelled" {
				return q.UpdateOrder(ctx, db.UpdateOrderParams{ID: order.ID, Status: string(order.Status), State: state})
			}
			return q.InsertOrder(ctx, db.InsertOrderParams{
				ID: order.ID, PointID: order.PointID, Status: string(order.Status), PlacedAt: order.PlacedAt, State: state,
			})
		},
		Reset: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `TRUNCATE orders_history`)
			return err
		},
	}
}

// orderEvents - события потока заказа; остальные события история пропускает.
var orderEvents = map[string]bool{"OrderAccepted": true, "OrderRejected": true, "OrderCancelled": true}

// orderIDOf - идентификатор заказа из имени его потока.
func orderIDOf(e eventstore.Event) uuid.UUID {
	return uuid.MustParse(strings.TrimPrefix(e.StreamID, "order-"))
}

// foldOrder собирает заказ из событий его потока. Это единственная свёртка заказа:
// по ней строятся и чтение по идентификатору, и история.
func foldOrder(stored []eventstore.Event) (Order, error) {
	var order Order
	for _, e := range stored {
		if err := applyOrderEvent(&order, e); err != nil {
			return Order{}, err
		}
	}
	return order, nil
}

// applyOrderEvent меняет заказ событием его потока; новое событие потока заказа обязано попасть сюда.
func applyOrderEvent(order *Order, e eventstore.Event) error {
	payload, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
	if err != nil {
		return err
	}
	switch p := payload.(type) {
	case OrderAccepted:
		*order = Order{
			ID: p.OrderID, PointID: p.PointID, Status: OrderAcceptedStatus, PlacedAt: e.OccurredAt.UTC(), Items: p.Items,
			PlacedBy: p.AcceptedBy,
		}
	case OrderRejected:
		*order = Order{
			ID: p.OrderID, PointID: p.PointID, Status: OrderRejectedStatus, PlacedAt: e.OccurredAt.UTC(),
			Items: p.Items, Shortages: p.Shortages, PlacedBy: p.RejectedBy,
		}
	case OrderCancelled:
		cancelledAt := e.OccurredAt.UTC()
		order.Status, order.CancelledAt = OrderCancelledStatus, &cancelledAt
		order.CancelledBy, order.CancelComment = &p.CancelledBy, p.Comment
	default:
		return fmt.Errorf("orders: order stream has unsupported event %s", e.Type)
	}
	return nil
}
