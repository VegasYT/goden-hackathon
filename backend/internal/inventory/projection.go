package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goden/backend/internal/inventory/db"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/projection"
)

const deliveryStreamPrefix = "delivery-"

// DeliveriesProjection ведёт таблицу inventory_deliveries для списка поставок.
//
// На любое событие потока поставки состояние собирается заново той же свёрткой, что и при чтении
// по идентификатору, и строка перезаписывается целиком: второй реализации правил распределения
// количества по позициям нет, и список не может разойтись с чтением поставки.
//
// Свёртка читает поток до текущего конца, поэтому строка может отражать события, до которых
// проекция ещё не дошла. Следующие события того же потока перезапишут её тем же значением.
func DeliveriesProjection() projection.Projection {
	return projection.Projection{
		// версия 3: в состоянии позиции появились карантин, его решения и исходное распределение
		Name: "inventory-deliveries", Version: 3,
		Apply: func(ctx context.Context, tx pgx.Tx, e eventstore.Event) error {
			// Событие выбирается по потоку, а не по типу: новое событие поставки подхватится само,
			// а если свёртка его не знает, она вернёт ошибку и проекция остановится.
			rawID, isDelivery := strings.CutPrefix(e.StreamID, deliveryStreamPrefix)
			if !isDelivery {
				return nil
			}
			id, err := uuid.Parse(rawID)
			if err != nil {
				return fmt.Errorf("inventory: delivery stream %q: %w", e.StreamID, err)
			}
			delivery, _, err := loadDelivery(ctx, tx, id)
			if err != nil {
				return err
			}
			state, err := json.Marshal(delivery)
			if err != nil {
				return fmt.Errorf("inventory: encode delivery %s: %w", id, err)
			}
			return db.New(tx).UpsertDelivery(ctx, db.UpsertDeliveryParams{
				ID: delivery.ID, PointID: delivery.PointID, Number: delivery.Number, Status: string(delivery.Status),
				DeliveredAt: delivery.DeliveredAt, OpenQuarantine: delivery.openQuarantineItems() > 0,
				OpenQuarantineItems: delivery.openQuarantineItems(), State: state,
			})
		},
		Reset: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `TRUNCATE inventory_deliveries`)
			return err
		},
	}
}

// openQuarantineItems - число позиций с карантинным количеством, по которому ещё нет решения.
func (d Delivery) openQuarantineItems() int32 {
	var open int32
	for _, item := range d.Items {
		if item.QuarantinedQuantity > 0 {
			open++
		}
	}
	return open
}
