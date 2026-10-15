// Package inventory ведёт поставки, партии, остатки и списания.
package inventory

import (
	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

func init() {
	eventstore.Register[DeliveryRegistered]()
	eventstore.Register[DeliveryApproved]()
	eventstore.Register[DeliveryApprovalCorrected]()
	eventstore.Register[QuarantinedDeliveryItemAccepted]()
	eventstore.Register[QuarantinedDeliveryItemRejected]()
	eventstore.Register[BatchReceived]()
	eventstore.Register[StockConsumed]()
	eventstore.Register[StockReturned]()
	eventstore.Register[StockAdjusted]()
	eventstore.Register[StockWrittenOff]()
	eventstore.Register[BatchExpiryWarningIssued]()
	eventstore.Register[BatchExpired]()
}

// DeliveryStream - поток поставки: регистрация, приёмка, корректировки, решения по карантину.
func DeliveryStream(deliveryID uuid.UUID) string { return "delivery-" + deliveryID.String() }

// StockStream - складской поток продукта на точке. В нём лежат события всех партий продукта
// на этой точке: партия не имеет собственного потока (ADR-002), а склад у каждой точки свой
// (ADR-013). По этому имени берутся блокировки, поэтому точки не ждут друг друга.
func StockStream(pointID, productID uuid.UUID) string {
	return stockStreamPrefix + pointID.String() + "-" + productID.String()
}

const stockStreamPrefix = "stock-"

// События потока поставки.

// DeliveryRegistered не меняет остаток: партии появляются только после приёмки (ADR-010).
type DeliveryRegistered struct {
	PointID      uuid.UUID        `json:"point_id"`
	DeliveryID   uuid.UUID        `json:"delivery_id"`
	Number       string           `json:"number"`
	DeliveredAt  eventstore.Time  `json:"delivered_at"`
	RegisteredBy eventstore.Actor `json:"registered_by"`
	Items        []DeliveryItem   `json:"items"`
}

func (DeliveryRegistered) EventType() string  { return "DeliveryRegistered" }
func (DeliveryRegistered) SchemaVersion() int { return 1 }

type DeliveryItem struct {
	ItemID           uuid.UUID        `json:"item_id"`
	ProductID        uuid.UUID        `json:"product_id"`
	DeclaredQuantity int64            `json:"declared_quantity"`
	ExpiresAt        eventstore.Time  `json:"expires_at"`
	ManufacturedAt   *eventstore.Time `json:"manufactured_at,omitempty"`
}

// DeliveryApproved - неизменяемый снимок приёмки со всеми решениями по позициям.
// Отдельных событий осмотра, отклонения и помещения в карантин нет.
type DeliveryApproved struct {
	// точка поставки: по ней сигнал об изменении находит подписчиков; у событий,
	// записанных до появления поля, она пустая
	PointID     uuid.UUID        `json:"point_id"`
	DeliveryID  uuid.UUID        `json:"delivery_id"`
	DeliveredAt eventstore.Time  `json:"delivered_at"`
	ApprovedAt  eventstore.Time  `json:"approved_at"`
	ApprovedBy  eventstore.Actor `json:"approved_by"`
	Items       []ApprovedItem   `json:"items"`
}

func (DeliveryApproved) EventType() string  { return "DeliveryApproved" }
func (DeliveryApproved) SchemaVersion() int { return 1 }

// ApprovedItem: заявлено = принято + отклонено + карантин. Равенство проверяет команда приёмки.
type ApprovedItem struct {
	ItemID              uuid.UUID       `json:"item_id"`
	ProductID           uuid.UUID       `json:"product_id"`
	DeclaredQuantity    int64           `json:"declared_quantity"`
	AcceptedQuantity    int64           `json:"accepted_quantity"`
	RejectedQuantity    int64           `json:"rejected_quantity"`
	QuarantinedQuantity int64           `json:"quarantined_quantity"`
	ExpiresAt           eventstore.Time `json:"expires_at"`
	// просрочена ли позиция на момент приёмки
	Expired bool `json:"expired"`
	// партия, созданная для принятого количества
	BatchID          *uuid.UUID `json:"batch_id,omitempty"`
	RejectReason     string     `json:"reject_reason,omitempty"`
	QuarantineReason string     `json:"quarantine_reason,omitempty"`
	Comment          string     `json:"comment,omitempty"`
}

// DeliveryApprovalCorrected исправляет приёмку, не переписывая исходный DeliveryApproved.
type DeliveryApprovalCorrected struct {
	// точка поставки, как в DeliveryApproved
	PointID      uuid.UUID        `json:"point_id"`
	CorrectionID uuid.UUID        `json:"correction_id"`
	DeliveryID   uuid.UUID        `json:"delivery_id"`
	Reason       string           `json:"reason"`
	CorrectedAt  eventstore.Time  `json:"corrected_at"`
	CorrectedBy  eventstore.Actor `json:"corrected_by"`
	Items        []CorrectedItem  `json:"items"`
}

func (DeliveryApprovalCorrected) EventType() string  { return "DeliveryApprovalCorrected" }
func (DeliveryApprovalCorrected) SchemaVersion() int { return 1 }

// CorrectedItem содержит итоговое распределение количества после корректировки, а не разницу.
type CorrectedItem struct {
	ItemID              uuid.UUID  `json:"item_id"`
	AcceptedQuantity    int64      `json:"accepted_quantity"`
	RejectedQuantity    int64      `json:"rejected_quantity"`
	QuarantinedQuantity int64      `json:"quarantined_quantity"`
	BatchID             *uuid.UUID `json:"batch_id,omitempty"`
}

type QuarantinedDeliveryItemAccepted struct {
	// точка поставки, как в DeliveryApproved
	PointID    uuid.UUID        `json:"point_id"`
	DeliveryID uuid.UUID        `json:"delivery_id"`
	ItemID     uuid.UUID        `json:"item_id"`
	Quantity   int64            `json:"quantity"`
	BatchID    uuid.UUID        `json:"batch_id"`
	Comment    string           `json:"comment,omitempty"`
	DecidedAt  eventstore.Time  `json:"decided_at"`
	DecidedBy  eventstore.Actor `json:"decided_by"`
}

func (QuarantinedDeliveryItemAccepted) EventType() string  { return "QuarantinedDeliveryItemAccepted" }
func (QuarantinedDeliveryItemAccepted) SchemaVersion() int { return 1 }

type QuarantinedDeliveryItemRejected struct {
	// точка поставки, как в DeliveryApproved
	PointID    uuid.UUID        `json:"point_id"`
	DeliveryID uuid.UUID        `json:"delivery_id"`
	ItemID     uuid.UUID        `json:"item_id"`
	Quantity   int64            `json:"quantity"`
	Reason     string           `json:"reason"`
	Comment    string           `json:"comment,omitempty"`
	DecidedAt  eventstore.Time  `json:"decided_at"`
	DecidedBy  eventstore.Actor `json:"decided_by"`
}

func (QuarantinedDeliveryItemRejected) EventType() string  { return "QuarantinedDeliveryItemRejected" }
func (QuarantinedDeliveryItemRejected) SchemaVersion() int { return 1 }

// События складского потока продукта.

// BatchSource - откуда появилась партия.
type BatchSource string

const (
	BatchFromApproval   BatchSource = "approval"
	BatchFromQuarantine BatchSource = "quarantine"
	BatchFromCorrection BatchSource = "correction"
)

func (s *BatchSource) UnmarshalText(text []byte) error {
	return eventstore.ParseEnum(s, text, "batch source", BatchFromApproval, BatchFromQuarantine, BatchFromCorrection)
}

// BatchReceived увеличивает доступный остаток продукта.
type BatchReceived struct {
	PointID    uuid.UUID       `json:"point_id"`
	BatchID    uuid.UUID       `json:"batch_id"`
	ProductID  uuid.UUID       `json:"product_id"`
	DeliveryID uuid.UUID       `json:"delivery_id"`
	ItemID     uuid.UUID       `json:"item_id"`
	Quantity   int64           `json:"quantity"`
	ExpiresAt  eventstore.Time `json:"expires_at"`
	ReceivedAt eventstore.Time `json:"received_at"`
	Source     BatchSource     `json:"source"`
}

func (BatchReceived) EventType() string  { return "BatchReceived" }
func (BatchReceived) SchemaVersion() int { return 1 }

// StockConsumed - списание продукта по заказу с разбивкой по партиям в порядке FEFO.
type StockConsumed struct {
	PointID   uuid.UUID       `json:"point_id"`
	ProductID uuid.UUID       `json:"product_id"`
	OrderID   uuid.UUID       `json:"order_id"`
	Batches   []BatchQuantity `json:"batches"`
}

func (StockConsumed) EventType() string  { return "StockConsumed" }
func (StockConsumed) SchemaVersion() int { return 1 }

// StockReturned - возврат продукта в партии при отмене заказа: ровно то, что взяли под него
// событием StockConsumed. В истёкшую партию возвращённое приходит просроченным.
type StockReturned struct {
	PointID    uuid.UUID        `json:"point_id"`
	ProductID  uuid.UUID        `json:"product_id"`
	OrderID    uuid.UUID        `json:"order_id"`
	Batches    []BatchQuantity  `json:"batches"`
	ReturnedBy eventstore.Actor `json:"returned_by"`
}

func (StockReturned) EventType() string  { return "StockReturned" }
func (StockReturned) SchemaVersion() int { return 1 }

type BatchQuantity struct {
	BatchID  uuid.UUID `json:"batch_id"`
	Quantity int64     `json:"quantity"`
}

// StockAdjusted - ручная корректировка остатка партии. Delta со знаком.
type StockAdjusted struct {
	PointID   uuid.UUID `json:"point_id"`
	ProductID uuid.UUID `json:"product_id"`
	BatchID   uuid.UUID `json:"batch_id"`
	Delta     int64     `json:"delta"`
	Reason    string    `json:"reason"`
	Comment   string    `json:"comment,omitempty"`
	// заполнено, если корректировка вызвана исправлением приёмки
	CorrectionID *uuid.UUID       `json:"correction_id,omitempty"`
	AdjustedBy   eventstore.Actor `json:"adjusted_by"`
}

func (StockAdjusted) EventType() string  { return "StockAdjusted" }
func (StockAdjusted) SchemaVersion() int { return 1 }

// WriteOffReason - причина списания (ADR-011).
type WriteOffReason string

const (
	WriteOffExpired          WriteOffReason = "expired"
	WriteOffTransportDamage  WriteOffReason = "transport_damage"
	WriteOffStorageViolation WriteOffReason = "storage_violation"
	WriteOffPhysicalDamage   WriteOffReason = "physical_damage"
	WriteOffQualityIssue     WriteOffReason = "quality_issue"
	WriteOffOther            WriteOffReason = "other"
)

var writeOffReasons = []WriteOffReason{
	WriteOffExpired, WriteOffTransportDamage, WriteOffStorageViolation,
	WriteOffPhysicalDamage, WriteOffQualityIssue, WriteOffOther,
}

func (r *WriteOffReason) UnmarshalText(text []byte) error {
	return eventstore.ParseEnum(r, text, "write-off reason", writeOffReasons...)
}

// StockWrittenOff - единственное событие, уменьшающее остаток при порче или истечении срока.
type StockWrittenOff struct {
	PointID      uuid.UUID        `json:"point_id"`
	ProductID    uuid.UUID        `json:"product_id"`
	BatchID      uuid.UUID        `json:"batch_id"`
	Quantity     int64            `json:"quantity"`
	Reason       WriteOffReason   `json:"reason"`
	Comment      string           `json:"comment,omitempty"`
	WrittenOffBy eventstore.Actor `json:"written_off_by"`
}

func (StockWrittenOff) EventType() string  { return "StockWrittenOff" }
func (StockWrittenOff) SchemaVersion() int { return 1 }

// BatchExpiryWarningIssued выдаётся один раз на партию и не меняет остаток (ADR-008).
type BatchExpiryWarningIssued struct {
	PointID              uuid.UUID       `json:"point_id"`
	ProductID            uuid.UUID       `json:"product_id"`
	BatchID              uuid.UUID       `json:"batch_id"`
	RemainingQuantity    int64           `json:"remaining_quantity"`
	ExpiresAt            eventstore.Time `json:"expires_at"`
	WarningPeriodSeconds int64           `json:"warning_period_seconds"`
}

func (BatchExpiryWarningIssued) EventType() string  { return "BatchExpiryWarningIssued" }
func (BatchExpiryWarningIssued) SchemaVersion() int { return 1 }

// BatchExpired - срок годности партии с остатком истёк. Выдаётся один раз на партию и не меняет
// остаток: просроченный остаток списывает вручную управляющий.
type BatchExpired struct {
	PointID           uuid.UUID       `json:"point_id"`
	ProductID         uuid.UUID       `json:"product_id"`
	BatchID           uuid.UUID       `json:"batch_id"`
	RemainingQuantity int64           `json:"remaining_quantity"`
	ExpiresAt         eventstore.Time `json:"expires_at"`
}

func (BatchExpired) EventType() string  { return "BatchExpired" }
func (BatchExpired) SchemaVersion() int { return 1 }
