package inventory_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/inventory"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/eventstore/estest"
)

func id(n byte) uuid.UUID {
	return uuid.MustParse("0198c0de-0000-7000-8000-0000000000" + string("0123456789abcdef"[n>>4]) + string("0123456789abcdef"[n&0xf]))
}

var (
	productID    = id(1)
	deliveryID   = id(2)
	itemID       = id(3)
	batchID      = id(4)
	secondBatch  = id(5)
	orderID      = id(6)
	correctionID = id(7)

	// point - точка, на которой тесты пакета ведут склад, и её компания
	point = organization.Point{ID: id(8), CompanyID: id(9), Active: true}

	manager   = eventstore.Actor{Type: eventstore.ActorUser, ID: "4f1c2a7e-manager"}
	generator = eventstore.Actor{Type: eventstore.ActorService, ID: "service-account-generator"}

	deliveredAt = eventstore.At(time.Date(2026, 10, 10, 8, 30, 0, 0, time.UTC))
	approvedAt  = eventstore.At(time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC))
	expiresAt   = eventstore.At(time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	madeAt      = eventstore.At(time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC))
)

func approved(by eventstore.Actor) inventory.DeliveryApproved {
	return inventory.DeliveryApproved{
		DeliveryID: deliveryID, DeliveredAt: deliveredAt, ApprovedAt: approvedAt, ApprovedBy: by,
		Items: []inventory.ApprovedItem{{
			ItemID: itemID, ProductID: productID,
			DeclaredQuantity: 10, AcceptedQuantity: 6, RejectedQuantity: 3, QuarantinedQuantity: 1,
			ExpiresAt: expiresAt, BatchID: &batchID,
			RejectReason: "damaged_package", QuarantineReason: "temperature_check",
			Comment: "три упаковки вскрыты, одна тёплая",
		}},
	}
}

func TestEventContract(t *testing.T) {
	samples := []eventstore.Payload{
		inventory.DeliveryRegistered{
			PointID: point.ID, DeliveryID: deliveryID, Number: "ПН-000123", DeliveredAt: deliveredAt, RegisteredBy: manager,
			Items: []inventory.DeliveryItem{{
				ItemID: itemID, ProductID: productID, DeclaredQuantity: 10,
				ExpiresAt: expiresAt, ManufacturedAt: &madeAt,
			}},
		},
		approved(manager),
		inventory.DeliveryApprovalCorrected{
			CorrectionID: correctionID, DeliveryID: deliveryID, Reason: "ошибка пересчёта",
			CorrectedAt: approvedAt, CorrectedBy: manager,
			Items: []inventory.CorrectedItem{{
				ItemID: itemID, AcceptedQuantity: 7, RejectedQuantity: 2, QuarantinedQuantity: 1, BatchID: &batchID,
			}},
		},
		inventory.QuarantinedDeliveryItemAccepted{
			DeliveryID: deliveryID, ItemID: itemID, Quantity: 1, BatchID: secondBatch,
			Comment: "температура в норме", DecidedAt: approvedAt, DecidedBy: manager,
		},
		inventory.QuarantinedDeliveryItemRejected{
			DeliveryID: deliveryID, ItemID: itemID, Quantity: 1, Reason: "temperature_violation",
			Comment: "выше допустимой", DecidedAt: approvedAt, DecidedBy: manager,
		},
		inventory.BatchReceived{
			PointID: point.ID, BatchID: batchID, ProductID: productID, DeliveryID: deliveryID, ItemID: itemID,
			Quantity: 1500, ExpiresAt: expiresAt, ReceivedAt: approvedAt, Source: inventory.BatchFromApproval,
		},
		inventory.StockConsumed{PointID: point.ID, ProductID: productID, OrderID: orderID, Batches: []inventory.BatchQuantity{
			{BatchID: batchID, Quantity: 200},
			{BatchID: secondBatch, Quantity: 100},
		}},
		inventory.StockReturned{PointID: point.ID, ProductID: productID, OrderID: orderID, ReturnedBy: manager, Batches: []inventory.BatchQuantity{
			{BatchID: batchID, Quantity: 200},
			{BatchID: secondBatch, Quantity: 100},
		}},
		inventory.StockAdjusted{
			PointID: point.ID, ProductID: productID, BatchID: batchID, Delta: -50, Reason: "inventory_count",
			Comment: "пересчёт", CorrectionID: &correctionID, AdjustedBy: manager,
		},
		inventory.StockWrittenOff{
			PointID: point.ID, ProductID: productID, BatchID: batchID, Quantity: 300,
			Reason: inventory.WriteOffStorageViolation, Comment: "оставили вне холодильника", WrittenOffBy: manager,
		},
		inventory.BatchExpiryWarningIssued{
			PointID: point.ID, ProductID: productID, BatchID: batchID, RemainingQuantity: 1200,
			ExpiresAt: expiresAt, WarningPeriodSeconds: 86400,
		},
		inventory.BatchExpired{
			PointID: point.ID, ProductID: productID, BatchID: batchID, RemainingQuantity: 400, ExpiresAt: expiresAt,
		},
	}
	for _, sample := range samples {
		t.Run(sample.EventType(), func(t *testing.T) { estest.Golden(t, sample) })
	}
}

func TestStreamNames(t *testing.T) {
	// склад у каждой точки свой: точка входит в имя потока, по нему же берутся блокировки
	require.Equal(t, "stock-0198c0de-0000-7000-8000-000000000008-0198c0de-0000-7000-8000-000000000001", inventory.StockStream(point.ID, productID))
	require.Equal(t, "delivery-0198c0de-0000-7000-8000-000000000002", inventory.DeliveryStream(deliveryID))
}

func TestQuantityIsIntegerInJSON(t *testing.T) {
	raw, err := eventstore.Encode(inventory.BatchReceived{PointID: point.ID, Quantity: 1500, Source: inventory.BatchFromApproval})

	require.NoError(t, err)
	require.Contains(t, string(raw), `"quantity":1500,`)
}

func TestTimeIsUTCInJSON(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)
	event := inventory.BatchExpiryWarningIssued{
		PointID: point.ID, ExpiresAt: eventstore.Time{Time: time.Date(2026, 10, 15, 3, 0, 0, 0, moscow)},
	}

	raw, err := eventstore.Encode(event)

	require.NoError(t, err)
	require.Contains(t, string(raw), `"expires_at":"2026-10-15T00:00:00Z"`)
}

func TestApprovalActorsAreDistinguishable(t *testing.T) {
	byUser, err := eventstore.Encode(approved(manager))
	require.NoError(t, err)
	byService, err := eventstore.Encode(approved(generator))
	require.NoError(t, err)

	require.Contains(t, string(byUser), `"approved_by":{"type":"user","id":"4f1c2a7e-manager"}`)
	require.Contains(t, string(byService), `"approved_by":{"type":"service","id":"service-account-generator"}`)
}

func TestClosedSetsRejectUnknownValues(t *testing.T) {
	tests := []struct {
		eventType string
		payload   string
		unknown   string
	}{
		{"StockWrittenOff", `{"reason":"lost"}`, "lost"},
		{"BatchReceived", `{"source":"gift"}`, "gift"},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			_, err := eventstore.Decode(tt.eventType, 1, json.RawMessage(tt.payload))

			require.ErrorContains(t, err, tt.unknown)
		})
	}
}
