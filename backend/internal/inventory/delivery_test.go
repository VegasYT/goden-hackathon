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

var (
	now     = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	fresh   = now.Add(5 * 24 * time.Hour)
	stale   = now.Add(-time.Hour)
	manager = eventstore.Actor{Type: eventstore.ActorUser, ID: "user-1"}

	cream = catalog.Product{ID: uuid.Must(uuid.NewV7()), Name: "Сливки", Unit: catalog.UnitMilliliter, ShelfLifeDays: ptr(5)}
	salt  = catalog.Product{ID: uuid.Must(uuid.NewV7()), Name: "Соль", Unit: catalog.UnitGram}
	known = map[uuid.UUID]catalog.Product{cream.ID: cream, salt.ID: salt}
)

func ptr[T any](v T) *T { return &v }

func requireInvalid(t *testing.T, err error) {
	t.Helper()
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
}

func TestPlanRegistration(t *testing.T) {
	valid := func() NewDelivery {
		return NewDelivery{Number: "ПН-1", DeliveredAt: now, Items: []NewDeliveryItem{
			{ProductID: cream.ID, DeclaredQuantity: 1000, ExpiresAt: fresh},
		}}
	}

	tests := []struct {
		name   string
		change func(d *NewDelivery)
		valid  bool
	}{
		{"valid", func(*NewDelivery) {}, true},
		{"expired item is still registered", func(d *NewDelivery) { d.Items[0].ExpiresAt = stale }, true},
		{"manufactured within shelf life", func(d *NewDelivery) { d.Items[0].ManufacturedAt = ptr(fresh.Add(-5 * 24 * time.Hour)) }, true},
		{"no shelf life in catalog: dates are not compared", func(d *NewDelivery) {
			d.Items[0].ProductID = salt.ID
			d.Items[0].ManufacturedAt = ptr(fresh.Add(-365 * 24 * time.Hour))
		}, true},
		{"empty number", func(d *NewDelivery) { d.Number = "  " }, false},
		{"no delivery time", func(d *NewDelivery) { d.DeliveredAt = time.Time{} }, false},
		{"no items", func(d *NewDelivery) { d.Items = nil }, false},
		{"zero quantity", func(d *NewDelivery) { d.Items[0].DeclaredQuantity = 0 }, false},
		{"negative quantity", func(d *NewDelivery) { d.Items[0].DeclaredQuantity = -1 }, false},
		{"quantity at the limit", func(d *NewDelivery) { d.Items[0].DeclaredQuantity = catalog.MaxQuantity }, true},
		{"quantity above the limit", func(d *NewDelivery) { d.Items[0].DeclaredQuantity = catalog.MaxQuantity + 1 }, false},
		{"no expires_at", func(d *NewDelivery) { d.Items[0].ExpiresAt = time.Time{} }, false},
		{"manufactured after expiry", func(d *NewDelivery) { d.Items[0].ManufacturedAt = ptr(fresh.Add(time.Hour)) }, false},
		{"longer than shelf life", func(d *NewDelivery) { d.Items[0].ManufacturedAt = ptr(fresh.Add(-10 * 24 * time.Hour)) }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.change(&in)

			event, err := planRegistration(uuid.Must(uuid.NewV7()), in, manager, known)

			if !tt.valid {
				requireInvalid(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, event.Items, len(in.Items))
			require.NotEqual(t, uuid.Nil, event.Items[0].ItemID)
		})
	}
}

func TestPlanRegistrationUnknownProduct(t *testing.T) {
	unknown := uuid.Must(uuid.NewV7())
	in := NewDelivery{Number: "ПН-1", DeliveredAt: now, Items: []NewDeliveryItem{
		{ProductID: unknown, DeclaredQuantity: 1, ExpiresAt: fresh},
	}}

	_, err := planRegistration(uuid.Must(uuid.NewV7()), in, manager, known)

	var unknownProduct *catalog.UnknownProductError
	require.ErrorAs(t, err, &unknownProduct)
	require.Equal(t, unknown, unknownProduct.ProductID)
}

// registered возвращает зарегистрированную поставку из двух позиций по 10: годной и с заданным сроком.
func registered(t *testing.T, secondExpiresAt time.Time) Delivery {
	t.Helper()
	event, err := planRegistration(uuid.Must(uuid.NewV7()), NewDelivery{
		Number: "ПН-1", DeliveredAt: now.Add(-time.Hour), Items: []NewDeliveryItem{
			{ProductID: cream.ID, DeclaredQuantity: 10, ExpiresAt: fresh},
			{ProductID: salt.ID, DeclaredQuantity: 10, ExpiresAt: secondExpiresAt},
		},
	}, manager, known)
	require.NoError(t, err)
	d, err := foldDelivery([]eventstore.Payload{event})
	require.NoError(t, err)
	return d
}

func acceptAll(item DeliveryItemState) ItemDecision {
	return ItemDecision{ItemID: item.ID, AcceptedQuantity: item.DeclaredQuantity}
}

func rejectAll(item DeliveryItemState) ItemDecision {
	return ItemDecision{ItemID: item.ID, RejectedQuantity: item.DeclaredQuantity, RejectReason: "expired", Comment: "срок истёк"}
}

// Сумма трёх частей переполняет int64 и совпадает с заявленным количеством 10:
// без проверки каждой части приёмка создавала партию на 9223372036854775807 единиц.
func TestPlanApprovalRejectsOverflowingDecision(t *testing.T) {
	d := registered(t, fresh)
	overflow := ItemDecision{
		ItemID: d.Items[0].ID, AcceptedQuantity: math.MaxInt64, RejectedQuantity: math.MaxInt64, QuarantinedQuantity: 12,
		RejectReason: "damaged", QuarantineReason: "check", Comment: "c",
	}
	require.Equal(t, d.Items[0].DeclaredQuantity, overflow.AcceptedQuantity+overflow.RejectedQuantity+overflow.QuarantinedQuantity,
		"the sum wraps around to the declared quantity")

	_, batches, err := d.planApproval([]ItemDecision{overflow, acceptAll(d.Items[1])}, manager, now)

	requireInvalid(t, err)
	require.Empty(t, batches)
}

func TestPlanApprovalFull(t *testing.T) {
	d := registered(t, fresh)

	approved, batches, err := d.planApproval([]ItemDecision{acceptAll(d.Items[0]), acceptAll(d.Items[1])}, manager, now)

	require.NoError(t, err)
	require.Equal(t, manager, approved.ApprovedBy)
	require.Equal(t, now, approved.ApprovedAt.Time)
	require.Equal(t, d.DeliveredAt, approved.DeliveredAt.Time)
	require.Len(t, batches, 2)
	for i, batch := range batches {
		require.Equal(t, d.Items[i].ID, batch.ItemID)
		require.Equal(t, d.Items[i].ProductID, batch.ProductID)
		require.Equal(t, int64(10), batch.Quantity)
		require.Equal(t, BatchFromApproval, batch.Source)
		require.Equal(t, now, batch.ReceivedAt.Time)
		require.Equal(t, batch.BatchID, *approved.Items[i].BatchID)
	}
}

func TestPlanApprovalPartial(t *testing.T) {
	d := registered(t, fresh)
	partial := ItemDecision{
		ItemID: d.Items[0].ID, AcceptedQuantity: 6, RejectedQuantity: 3, QuarantinedQuantity: 1,
		RejectReason: "damaged_package", QuarantineReason: "temperature_check", Comment: "три вскрыты, одна тёплая",
	}

	approved, batches, err := d.planApproval([]ItemDecision{partial, rejectAll(d.Items[1])}, manager, now)

	require.NoError(t, err)
	require.Len(t, batches, 1, "only the accepted quantity becomes a batch")
	require.Equal(t, int64(6), batches[0].Quantity)

	first := approved.Items[0]
	require.Equal(t, []int64{10, 6, 3, 1},
		[]int64{first.DeclaredQuantity, first.AcceptedQuantity, first.RejectedQuantity, first.QuarantinedQuantity})
	require.Equal(t, "damaged_package", first.RejectReason)
	require.Equal(t, "temperature_check", first.QuarantineReason)
	require.Equal(t, "три вскрыты, одна тёплая", first.Comment)
	require.NotNil(t, first.BatchID)

	second := approved.Items[1]
	require.Nil(t, second.BatchID, "fully rejected item has no batch")
	require.Equal(t, int64(10), second.RejectedQuantity)
}

func TestPlanApprovalRejected(t *testing.T) {
	d := registered(t, fresh)
	first, second := d.Items[0], d.Items[1]
	ok := acceptAll(second)

	tests := []struct {
		name      string
		decisions []ItemDecision
	}{
		{"sum does not match", []ItemDecision{{ItemID: first.ID, AcceptedQuantity: 6, RejectedQuantity: 3, RejectReason: "r", Comment: "c"}, ok}},
		{"negative quantity", []ItemDecision{{ItemID: first.ID, AcceptedQuantity: 11, RejectedQuantity: -1}, ok}},
		{"rejected without reason", []ItemDecision{{ItemID: first.ID, AcceptedQuantity: 7, RejectedQuantity: 3, Comment: "c"}, ok}},
		{"rejected without comment", []ItemDecision{{ItemID: first.ID, AcceptedQuantity: 7, RejectedQuantity: 3, RejectReason: "r"}, ok}},
		{"quarantined without reason", []ItemDecision{{ItemID: first.ID, AcceptedQuantity: 9, QuarantinedQuantity: 1, Comment: "c"}, ok}},
		{"quarantined without comment", []ItemDecision{{ItemID: first.ID, AcceptedQuantity: 9, QuarantinedQuantity: 1, QuarantineReason: "r"}, ok}},
		{"not all items decided", []ItemDecision{acceptAll(first)}},
		{"unknown item", []ItemDecision{acceptAll(first), ok, {ItemID: uuid.Must(uuid.NewV7()), AcceptedQuantity: 1}}},
		{"item decided twice", []ItemDecision{acceptAll(first), acceptAll(first), ok}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, batches, err := d.planApproval(tt.decisions, manager, now)

			requireInvalid(t, err)
			require.Empty(t, batches)
		})
	}
}

func TestPlanApprovalExpiredItem(t *testing.T) {
	d := registered(t, stale)

	t.Run("cannot be accepted", func(t *testing.T) {
		_, _, err := d.planApproval([]ItemDecision{acceptAll(d.Items[0]), acceptAll(d.Items[1])}, manager, now)

		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		require.Contains(t, validation.Field, d.Items[1].ID.String(), "the error names the item")
	})

	t.Run("can be rejected and is marked expired", func(t *testing.T) {
		approved, batches, err := d.planApproval([]ItemDecision{acceptAll(d.Items[0]), rejectAll(d.Items[1])}, manager, now)

		require.NoError(t, err)
		require.False(t, approved.Items[0].Expired)
		require.True(t, approved.Items[1].Expired)
		require.Len(t, batches, 1)
	})

	t.Run("can be quarantined", func(t *testing.T) {
		quarantine := ItemDecision{ItemID: d.Items[1].ID, QuarantinedQuantity: 10, QuarantineReason: "label_check", Comment: "дата нечитаема"}

		_, _, err := d.planApproval([]ItemDecision{acceptAll(d.Items[0]), quarantine}, manager, now)

		require.NoError(t, err)
	})

	t.Run("expiry at the very moment of approval counts as expired", func(t *testing.T) {
		atExpiry := registered(t, now)

		_, _, err := atExpiry.planApproval([]ItemDecision{acceptAll(atExpiry.Items[0]), acceptAll(atExpiry.Items[1])}, manager, now)

		requireInvalid(t, err)
	})
}

func TestPlanApprovalOnce(t *testing.T) {
	d := registered(t, fresh)
	decisions := []ItemDecision{acceptAll(d.Items[0]), acceptAll(d.Items[1])}
	approved, _, err := d.planApproval(decisions, manager, now)
	require.NoError(t, err)
	d, err = d.apply(approved)
	require.NoError(t, err)

	_, _, err = d.planApproval(decisions, manager, now)

	require.ErrorIs(t, err, ErrAlreadyApproved)
}

func TestFoldDelivery(t *testing.T) {
	d := registered(t, fresh)
	require.Equal(t, DeliveryRegisteredStatus, d.Status)
	require.Nil(t, d.ApprovedAt)
	require.Zero(t, d.Items[0].AcceptedQuantity)

	registeredItems := d.Items
	approved, _, err := d.planApproval([]ItemDecision{
		{
			ItemID: d.Items[0].ID, AcceptedQuantity: 6, RejectedQuantity: 3, QuarantinedQuantity: 1,
			RejectReason: "damaged_package", QuarantineReason: "temperature_check", Comment: "c",
		},
		acceptAll(d.Items[1]),
	}, manager, now)
	require.NoError(t, err)

	after, err := d.apply(approved)
	require.NoError(t, err)

	require.Equal(t, DeliveryApprovedStatus, after.Status)
	require.Equal(t, now, *after.ApprovedAt)
	require.Equal(t, manager, *after.ApprovedBy)
	for _, item := range after.Items {
		require.Equal(t, item.DeclaredQuantity, item.AcceptedQuantity+item.RejectedQuantity+item.QuarantinedQuantity)
	}
	require.Equal(t, int64(6), after.Items[0].AcceptedQuantity)
	require.Equal(t, []uuid.UUID{*approved.Items[0].BatchID}, after.Items[0].BatchIDs)
	require.Zero(t, registeredItems[0].AcceptedQuantity, "apply must not change the previous state")
}

func TestFoldDeliveryRejectsUnsupportedEvent(t *testing.T) {
	_, err := foldDelivery([]eventstore.Payload{StockConsumed{}})

	require.ErrorContains(t, err, "StockConsumed")
}
