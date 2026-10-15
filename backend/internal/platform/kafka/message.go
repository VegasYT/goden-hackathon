// Package kafka связывает Event Store с Kafka.
//
// Event Store остаётся источником истины. Publisher подтверждает свой checkpoint
// только после ack broker, а Consumer устраняет повторную доставку по event_id.
package kafka

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

const (
	EnvelopeVersion = 1
	CatalogTopic    = "catalog.v1"
	DeliveriesTopic = "deliveries.v1"
	StockTopic      = "stock.v1"
	OrdersTopic     = "orders.v1"
	ExpiryTopic     = "expiry.v1"
	ErrorsTopic     = "errors.v1"
)

// Envelope - версионированный интеграционный конверт. Payload и Metadata сохраняются
// без преобразования, поэтому consumers могут развиваться независимо от publisher.
type Envelope struct {
	EnvelopeVersion int             `json:"envelope_version"`
	EventID         uuid.UUID       `json:"event_id"`
	EventType       string          `json:"event_type"`
	EventVersion    int             `json:"event_version"`
	StreamID        string          `json:"stream_id"`
	StreamVersion   int64           `json:"stream_version"`
	Payload         json.RawMessage `json:"payload"`
	Metadata        json.RawMessage `json:"metadata"`
	CorrelationID   *uuid.UUID      `json:"correlation_id,omitempty"`
	CausationID     *uuid.UUID      `json:"causation_id,omitempty"`
	OccurredAt      time.Time       `json:"occurred_at"`
}

// ErrorEnvelope описывает сообщение, которое consumer не может обработать повтором.
type ErrorEnvelope struct {
	Consumer    string    `json:"consumer"`
	SourceTopic string    `json:"source_topic"`
	EventID     uuid.UUID `json:"event_id"`
	EventType   string    `json:"event_type"`
	Error       string    `json:"error"`
	Original    []byte    `json:"original"`
	FailedAt    time.Time `json:"failed_at"`
}

// Message не привязывает publisher и тесты к конкретному Kafka-клиенту.
type Message struct {
	Topic string
	Key   []byte
	Value []byte
}

func envelope(e eventstore.Event) Envelope {
	result := Envelope{
		EnvelopeVersion: EnvelopeVersion,
		EventID:         e.ID, EventType: e.Type, EventVersion: e.SchemaVersion,
		StreamID: e.StreamID, StreamVersion: e.Version, Payload: e.Payload,
		Metadata: e.Metadata, OccurredAt: e.OccurredAt,
	}
	if e.CorrelationID != uuid.Nil {
		id := e.CorrelationID
		result.CorrelationID = &id
	}
	if e.CausationID != uuid.Nil {
		id := e.CausationID
		result.CausationID = &id
	}
	return result
}

func topicFor(eventType string) (string, bool) {
	switch eventType {
	case "ProductCreated", "DishCreated", "RecipeChanged", "DishAvailabilityChanged",
		"ProductChanged", "ProductArchived", "ProductRestored", "DishRenamed", "DishArchived", "DishRestored",
		"HotOfferChanged":
		return CatalogTopic, true
	case "DeliveryRegistered", "DeliveryApproved", "DeliveryApprovalCorrected",
		"QuarantinedDeliveryItemAccepted", "QuarantinedDeliveryItemRejected":
		return DeliveriesTopic, true
	case "BatchReceived", "StockConsumed", "StockReturned", "StockAdjusted", "StockWrittenOff":
		return StockTopic, true
	case "OrderAccepted", "OrderRejected", "OrderCancelled":
		return OrdersTopic, true
	case "BatchExpiryWarningIssued", "BatchExpired":
		return ExpiryTopic, true
	default:
		return "", false
	}
}

func fullTopic(prefix, topic string) string {
	prefix = strings.TrimSuffix(prefix, ".")
	if prefix == "" {
		return topic
	}
	return prefix + "." + topic
}

// FullTopic добавляет общий префикс окружения к логическому имени Kafka-топика.
func FullTopic(prefix, topic string) string { return fullTopic(prefix, topic) }

func marshalEvent(e eventstore.Event, prefix string) (Message, bool, error) {
	topic, publish := topicFor(e.Type)
	if !publish {
		return Message{}, false, nil
	}
	raw, err := json.Marshal(envelope(e))
	if err != nil {
		return Message{}, false, fmt.Errorf("kafka: encode event %s: %w", e.ID, err)
	}
	return Message{Topic: fullTopic(prefix, topic), Key: []byte(e.StreamID), Value: raw}, true, nil
}
