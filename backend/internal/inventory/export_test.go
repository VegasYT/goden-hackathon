package inventory

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goden/backend/internal/inventory/db"
	"goden/backend/internal/platform/eventstore"
)

// Тестовая опора: перестроение таблицы остатков из Event Store. В сборку процессов не попадает.
// На работающей системе так делать нельзя: перестроение требует остановки записи.

type replayer func(ctx context.Context, q *db.Queries, p eventstore.Payload, occurredAt time.Time) error

// Replayers - что делает с таблицами записи склада (остатки и продукты из поставок) каждое событие
// модуля; nil - событие их не меняет.
// Событие модуля, которого здесь нет, перестроение считает ошибкой: новое складское событие
// нельзя пропустить молча.
var Replayers = map[string]replayer{
	"BatchReceived": func(ctx context.Context, q *db.Queries, p eventstore.Payload, _ time.Time) error {
		return applyBatchReceived(ctx, q, p.(BatchReceived))
	},
	"StockConsumed": func(ctx context.Context, q *db.Queries, p eventstore.Payload, _ time.Time) error {
		return applyStockConsumed(ctx, q, p.(StockConsumed))
	},
	"StockReturned": func(ctx context.Context, q *db.Queries, p eventstore.Payload, _ time.Time) error {
		return applyStockReturned(ctx, q, p.(StockReturned))
	},
	"StockWrittenOff": func(ctx context.Context, q *db.Queries, p eventstore.Payload, _ time.Time) error {
		return applyStockWrittenOff(ctx, q, p.(StockWrittenOff))
	},
	"StockAdjusted": func(ctx context.Context, q *db.Queries, p eventstore.Payload, _ time.Time) error {
		return applyStockAdjusted(ctx, q, p.(StockAdjusted))
	},
	"BatchExpiryWarningIssued": func(ctx context.Context, q *db.Queries, p eventstore.Payload, occurredAt time.Time) error {
		return applyBatchExpiryWarningIssued(ctx, q, p.(BatchExpiryWarningIssued), occurredAt)
	},
	"BatchExpired": func(ctx context.Context, q *db.Queries, p eventstore.Payload, occurredAt time.Time) error {
		return applyBatchExpired(ctx, q, p.(BatchExpired), occurredAt)
	},
	// события поставки: остаток меняют сопровождающие их BatchReceived и StockAdjusted
	"DeliveryRegistered": func(ctx context.Context, q *db.Queries, p eventstore.Payload, _ time.Time) error {
		return applyDeliveryRegistered(ctx, q, p.(DeliveryRegistered))
	},
	"DeliveryApproved":                nil,
	"DeliveryApprovalCorrected":       nil,
	"QuarantinedDeliveryItemAccepted": nil,
	"QuarantinedDeliveryItemRejected": nil,
}

// OwnsEvent сообщает, объявлено ли событие в этом модуле.
func OwnsEvent(p eventstore.Payload) bool {
	return reflect.TypeOf(p).PkgPath() == reflect.TypeOf(BatchReceived{}).PkgPath()
}

// Rebuild очищает таблицу остатков и собирает её заново из всех событий в общем порядке чтения,
// теми же функциями apply*, которыми её меняют команды.
func Rebuild(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `DELETE FROM inventory_batches; DELETE FROM inventory_delivered_products`); err != nil {
		return err
	}
	q := db.New(tx)
	var after eventstore.Checkpoint
	for {
		events, err := eventstore.ReadAll(ctx, tx, after, 500)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		for _, e := range events {
			after = e.Checkpoint()
			p, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
			if err != nil {
				return err
			}
			if !OwnsEvent(p) {
				continue
			}
			replay, known := Replayers[e.Type]
			if !known {
				return fmt.Errorf("inventory: rebuild does not know event %s", e.Type)
			}
			if replay == nil {
				continue
			}
			if err := replay(ctx, q, p, e.OccurredAt); err != nil {
				return fmt.Errorf("inventory: replay %s at position %d: %w", e.Type, e.Position, err)
			}
		}
	}
}

// MovementFromEvent открывает тестам перевод события складского потока в движение.
var MovementFromEvent = movementFromEvent

// FailOn заставляет проверку сроков возвращать ошибку для одной партии: так тест проверяет,
// что сбой по одной партии не мешает остальным.
func (c *ExpiryChecker) FailOn(batchID *uuid.UUID) {
	markExpired := c.markExpired
	c.markExpired = func(ctx context.Context, id uuid.UUID) (bool, error) {
		if batchID != nil && id == *batchID {
			return false, fmt.Errorf("injected failure for batch %s", id)
		}
		return markExpired(ctx, id)
	}
}
