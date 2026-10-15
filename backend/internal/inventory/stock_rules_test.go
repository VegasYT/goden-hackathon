package inventory

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
)

// goodBatch - годная партия с остатком 100, срок через 5 дней после now.
func goodBatch() Batch {
	return Batch{ID: uuid.Must(uuid.NewV7()), ProductID: cream.ID, Quantity: 100, ExpiresAt: fresh}
}

func expiredBatch() Batch {
	b := goodBatch()
	b.ExpiresAt = stale
	return b
}

func TestPlanWriteOff(t *testing.T) {
	tests := []struct {
		name  string
		batch Batch
		in    WriteOff
		valid bool
	}{
		{"partial", goodBatch(), WriteOff{Quantity: 30, Reason: WriteOffStorageViolation, Comment: "вне холодильника"}, true},
		{"whole quantity", goodBatch(), WriteOff{Quantity: 100, Reason: WriteOffPhysicalDamage, Comment: "упала коробка"}, true},
		{"expired batch with reason expired", expiredBatch(), WriteOff{Quantity: 100, Reason: WriteOffExpired, Comment: "срок истёк"}, true},
		{"expired batch with another reason", expiredBatch(), WriteOff{Quantity: 10, Reason: WriteOffPhysicalDamage, Comment: "разбито"}, true},
		{"zero quantity", goodBatch(), WriteOff{Quantity: 0, Reason: WriteOffOther, Comment: "c"}, false},
		{"negative quantity", goodBatch(), WriteOff{Quantity: -1, Reason: WriteOffOther, Comment: "c"}, false},
		{"more than the batch has", goodBatch(), WriteOff{Quantity: 101, Reason: WriteOffOther, Comment: "c"}, false},
		{"unknown reason", goodBatch(), WriteOff{Quantity: 1, Reason: "lost", Comment: "c"}, false},
		{"no comment", goodBatch(), WriteOff{Quantity: 1, Reason: WriteOffOther, Comment: "  "}, false},
		{"reason expired for a good batch", goodBatch(), WriteOff{Quantity: 1, Reason: WriteOffExpired, Comment: "c"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := planWriteOff(tt.batch, tt.in, manager, now)

			if !tt.valid {
				requireInvalid(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, StockWrittenOff{
				ProductID: tt.batch.ProductID, BatchID: tt.batch.ID, Quantity: tt.in.Quantity,
				Reason: tt.in.Reason, Comment: tt.in.Comment, WrittenOffBy: manager,
			}, event)
		})
	}
}

func TestPlanExpiredNotice(t *testing.T) {
	atExpiry, empty, noticed := goodBatch(), expiredBatch(), expiredBatch()
	atExpiry.ExpiresAt = now
	empty.Quantity = 0
	earlier := now.Add(-time.Hour)
	noticed.ExpiredNoticedAt = &earlier

	tests := []struct {
		name  string
		batch Batch
		due   bool
	}{
		{"expired batch with stock", expiredBatch(), true},
		{"batch expiring right now", atExpiry, true},
		{"good batch", goodBatch(), false},
		{"expired but already empty", empty, false},
		{"already noticed", noticed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, due := planExpiredNotice(tt.batch, now)

			require.Equal(t, tt.due, due)
			if due {
				require.Equal(t, BatchExpired{
					ProductID: tt.batch.ProductID, BatchID: tt.batch.ID, RemainingQuantity: tt.batch.Quantity,
					ExpiresAt: eventstore.At(tt.batch.ExpiresAt),
				}, event, "the notice carries the whole remaining quantity")
			}
		})
	}
}

func TestPlanAdjustment(t *testing.T) {
	tests := []struct {
		name  string
		in    Adjustment
		valid bool
	}{
		{"decrease", Adjustment{Delta: -50, Reason: "inventory_count"}, true},
		{"increase", Adjustment{Delta: 20, Reason: "inventory_count", Comment: "нашли коробку"}, true},
		{"decrease to zero", Adjustment{Delta: -100, Reason: "inventory_count"}, true},
		{"zero delta", Adjustment{Delta: 0, Reason: "inventory_count"}, false},
		{"no reason", Adjustment{Delta: -1, Reason: " "}, false},
		{"decrease below zero", Adjustment{Delta: -101, Reason: "inventory_count"}, false},
		{"increase up to the limit", Adjustment{Delta: catalog.MaxQuantity - 100, Reason: "inventory_count"}, true},
		{"increase above the limit", Adjustment{Delta: catalog.MaxQuantity - 99, Reason: "inventory_count"}, false},
		{"delta that overflowed the quantity", Adjustment{Delta: math.MaxInt64, Reason: "inventory_count"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := goodBatch()

			event, err := planAdjustment(batch, tt.in, manager)

			if !tt.valid {
				requireInvalid(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, StockAdjusted{
				ProductID: batch.ProductID, BatchID: batch.ID, Delta: tt.in.Delta,
				Reason: tt.in.Reason, Comment: tt.in.Comment, AdjustedBy: manager,
			}, event)
		})
	}
}

func TestPlanExpiryWarning(t *testing.T) {
	const day = 24 * time.Hour
	expiringIn := func(d time.Duration) Batch {
		b := goodBatch()
		b.ExpiresAt = now.Add(d)
		return b
	}
	empty, warned := expiringIn(20*time.Hour), expiringIn(20*time.Hour)
	empty.Quantity = 0
	warned.ExpiryWarnedAt = ptr(now.Add(-time.Hour))

	tests := []struct {
		name   string
		batch  Batch
		period time.Duration
		due    bool
	}{
		{"inside the period", expiringIn(20 * time.Hour), day, true},
		{"exactly at the period boundary", expiringIn(day), day, true},
		{"too early", expiringIn(5 * day), day, false},
		{"already expired", expiredBatch(), day, false},
		{"expiring right now", expiringIn(0), day, false},
		{"empty batch", empty, day, false},
		{"already warned", warned, day, false},
		{"already warned, longer period", warned, 2 * day, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, due := planExpiryWarning(tt.batch, tt.period, now)

			require.Equal(t, tt.due, due)
			if due {
				require.Equal(t, BatchExpiryWarningIssued{
					ProductID: tt.batch.ProductID, BatchID: tt.batch.ID, RemainingQuantity: 100,
					ExpiresAt: eventstore.At(tt.batch.ExpiresAt), WarningPeriodSeconds: 86400,
				}, event)
			}
		})
	}
}
