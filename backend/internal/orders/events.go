// Package orders оформляет заказы.
package orders

import (
	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

func init() {
	eventstore.Register[OrderAccepted]()
	eventstore.Register[OrderRejected]()
	eventstore.Register[OrderCancelled]()
}

// OrderStream - поток заказа.
func OrderStream(orderID uuid.UUID) string { return "order-" + orderID.String() }

// OrderAccepted не дублирует разбивку по партиям: она лежит в событиях StockConsumed
// складских потоков, записанных в той же транзакции, и связана через order_id.
type OrderAccepted struct {
	// точка, на которой оформлен заказ: расходуются только её партии (ADR-013)
	PointID    uuid.UUID        `json:"point_id"`
	OrderID    uuid.UUID        `json:"order_id"`
	AcceptedBy eventstore.Actor `json:"accepted_by"`
	Items      []OrderItem      `json:"items"`
}

func (OrderAccepted) EventType() string  { return "OrderAccepted" }
func (OrderAccepted) SchemaVersion() int { return 1 }

// OrderRejected - отказ из-за нехватки продуктов. Остатки при отказе не меняются.
type OrderRejected struct {
	PointID    uuid.UUID        `json:"point_id"`
	OrderID    uuid.UUID        `json:"order_id"`
	RejectedBy eventstore.Actor `json:"rejected_by"`
	Items      []OrderItem      `json:"items"`
	// по недостающим продуктам строится статистика дефицита
	Shortages []Shortage `json:"shortages"`
}

func (OrderRejected) EventType() string  { return "OrderRejected" }
func (OrderRejected) SchemaVersion() int { return 1 }

// OrderCancelled - отмена принятого заказа. Продукты возвращают события StockReturned
// складских потоков, записанные в той же транзакции и связанные через order_id.
type OrderCancelled struct {
	PointID     uuid.UUID        `json:"point_id"`
	OrderID     uuid.UUID        `json:"order_id"`
	CancelledBy eventstore.Actor `json:"cancelled_by"`
	// Comment - необязательный комментарий сотрудника; у событий до его появления пуст
	Comment string `json:"comment,omitempty"`
}

func (OrderCancelled) EventType() string  { return "OrderCancelled" }
func (OrderCancelled) SchemaVersion() int { return 1 }

type OrderItem struct {
	DishID   uuid.UUID `json:"dish_id"`
	Portions int64     `json:"portions"`
}

type Shortage struct {
	ProductID         uuid.UUID `json:"product_id"`
	RequiredQuantity  int64     `json:"required_quantity"`
	AvailableQuantity int64     `json:"available_quantity"`
}
