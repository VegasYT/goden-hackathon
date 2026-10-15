package inventory

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

// approvedDelivery возвращает принятую поставку из двух позиций по 10:
// первая - принято 6, в карантине 4; вторая - принята целиком.
// secondExpiresAt - срок годности второй позиции; срок первой - fresh.
func approvedDelivery(t *testing.T, secondExpiresAt time.Time) Delivery {
	t.Helper()
	d := registered(t, secondExpiresAt)
	approved, _, err := d.planApproval([]ItemDecision{
		{ItemID: d.Items[0].ID, AcceptedQuantity: 6, QuarantinedQuantity: 4, QuarantineReason: "temperature_check", Comment: "тёплые"},
		acceptAll(d.Items[1]),
	}, manager, now.Add(-time.Hour))
	require.NoError(t, err)
	d, err = d.apply(approved)
	require.NoError(t, err)
	return d
}

func requireBalanced(t *testing.T, d Delivery) {
	t.Helper()
	for _, item := range d.Items {
		require.Equal(t, item.DeclaredQuantity, item.AcceptedQuantity+item.RejectedQuantity+item.QuarantinedQuantity,
			"declared = accepted + rejected + quarantined for item %s", item.ID)
	}
}

func applyAll(t *testing.T, d Delivery, events ...eventstore.Payload) Delivery {
	t.Helper()
	for _, event := range events {
		var err error
		d, err = d.apply(event)
		require.NoError(t, err)
	}
	return d
}

// Решение по карантину.

func TestPlanQuarantineResolution(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item := d.Items[0]

	tests := []struct {
		name         string
		in           QuarantineDecision
		wantEvents   []string
		wantBatch    int64 // 0 - партии нет
		wantAccepted int64
		wantRejected int64
	}{
		{
			"accept everything",
			QuarantineDecision{ItemID: item.ID, AcceptedQuantity: 4},
			[]string{"QuarantinedDeliveryItemAccepted"},
			4, 10, 0,
		},
		{
			"reject everything",
			QuarantineDecision{ItemID: item.ID, RejectedQuantity: 4, RejectReason: "temperature_violation", Comment: "выше нормы"},
			[]string{"QuarantinedDeliveryItemRejected"},
			0, 6, 4,
		},
		{
			"part is good, part is not",
			QuarantineDecision{ItemID: item.ID, AcceptedQuantity: 3, RejectedQuantity: 1, RejectReason: "damaged", Comment: "одна вздулась"},
			[]string{"QuarantinedDeliveryItemAccepted", "QuarantinedDeliveryItemRejected"},
			3, 9, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, batch, err := d.planQuarantineResolution(tt.in, manager, now)
			require.NoError(t, err)

			var types []string
			for _, e := range events {
				types = append(types, e.EventType())
			}
			require.Equal(t, tt.wantEvents, types)

			if tt.wantBatch == 0 {
				require.Nil(t, batch)
			} else {
				require.Equal(t, BatchReceived{
					BatchID: batch.BatchID, ProductID: item.ProductID, DeliveryID: d.ID, ItemID: item.ID,
					Quantity: tt.wantBatch, ExpiresAt: eventstore.At(item.ExpiresAt),
					ReceivedAt: eventstore.At(now), Source: BatchFromQuarantine,
				}, *batch)
				require.Equal(t, batch.BatchID, events[0].(QuarantinedDeliveryItemAccepted).BatchID)
			}

			after := applyAll(t, d, events...)
			requireBalanced(t, after)
			got := after.Items[0]
			require.Equal(t, []int64{tt.wantAccepted, tt.wantRejected, 0},
				[]int64{got.AcceptedQuantity, got.RejectedQuantity, got.QuarantinedQuantity})
			if tt.wantBatch > 0 {
				require.Equal(t, []uuid.UUID{item.BatchIDs[0], batch.BatchID}, got.BatchIDs)
			} else {
				require.Equal(t, item.BatchIDs, got.BatchIDs)
			}
			require.Len(t, d.Items[0].BatchIDs, 1, "apply must not change the previous state")
		})
	}
}

func TestPlanQuarantineResolutionRejected(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item, closed := d.Items[0], d.Items[1]

	t.Run("delivery is not approved", func(t *testing.T) {
		pending := registered(t, fresh)
		_, _, err := pending.planQuarantineResolution(QuarantineDecision{ItemID: pending.Items[0].ID, AcceptedQuantity: 1}, manager, now)
		require.ErrorIs(t, err, ErrNotApproved)
	})
	t.Run("item without quarantine", func(t *testing.T) {
		_, _, err := d.planQuarantineResolution(QuarantineDecision{ItemID: closed.ID, AcceptedQuantity: 1}, manager, now)
		require.ErrorIs(t, err, ErrQuarantineClosed)
	})
	t.Run("already resolved", func(t *testing.T) {
		events, _, err := d.planQuarantineResolution(QuarantineDecision{ItemID: item.ID, AcceptedQuantity: 4}, manager, now)
		require.NoError(t, err)
		resolved := applyAll(t, d, events...)

		_, _, err = resolved.planQuarantineResolution(QuarantineDecision{ItemID: item.ID, AcceptedQuantity: 4}, manager, now)
		require.ErrorIs(t, err, ErrQuarantineClosed)
	})

	invalid := map[string]QuarantineDecision{
		"unknown item":             {ItemID: uuid.Must(uuid.NewV7()), AcceptedQuantity: 4},
		"sum does not match":       {ItemID: item.ID, AcceptedQuantity: 2, RejectedQuantity: 1, RejectReason: "r", Comment: "c"},
		"negative quantity":        {ItemID: item.ID, AcceptedQuantity: 5, RejectedQuantity: -1},
		"rejected without reason":  {ItemID: item.ID, RejectedQuantity: 4, Comment: "c"},
		"rejected without comment": {ItemID: item.ID, RejectedQuantity: 4, RejectReason: "r"},
	}
	for name, in := range invalid {
		t.Run(name, func(t *testing.T) {
			events, batch, err := d.planQuarantineResolution(in, manager, now)
			requireInvalid(t, err)
			require.Empty(t, events)
			require.Nil(t, batch)
		})
	}
}

func TestPlanQuarantineResolutionOfExpiredItem(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item := d.Items[0]
	afterExpiry := item.ExpiresAt.Add(time.Minute)

	_, _, err := d.planQuarantineResolution(QuarantineDecision{ItemID: item.ID, AcceptedQuantity: 4}, manager, afterExpiry)
	requireInvalid(t, err)

	events, batch, err := d.planQuarantineResolution(
		QuarantineDecision{ItemID: item.ID, RejectedQuantity: 4, RejectReason: "expired", Comment: "срок истёк в карантине"}, manager, afterExpiry)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Nil(t, batch)
}

// Корректировка приёмки.

func TestPlanCorrectionIncreasesAccepted(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item := d.Items[0] // принято 6, карантин 4

	corrected, received, adjusted, err := d.planCorrection(ApprovalCorrection{
		Reason: " ошибка пересчёта ",
		Items:  []ItemCorrection{{ItemID: item.ID, AcceptedQuantity: 8, QuarantinedQuantity: 2}},
	}, nil, manager, now)

	require.NoError(t, err)
	require.Empty(t, adjusted)
	require.Len(t, received, 1)
	require.Equal(t, BatchReceived{
		BatchID: received[0].BatchID, ProductID: item.ProductID, DeliveryID: d.ID, ItemID: item.ID,
		Quantity: 2, ExpiresAt: eventstore.At(item.ExpiresAt), ReceivedAt: eventstore.At(now), Source: BatchFromCorrection,
	}, received[0])
	require.Equal(t, "ошибка пересчёта", corrected.Reason)
	require.Equal(t, manager, corrected.CorrectedBy)
	require.Equal(t, []CorrectedItem{{
		ItemID: item.ID, AcceptedQuantity: 8, QuarantinedQuantity: 2, BatchID: &received[0].BatchID,
	}}, corrected.Items)

	after := applyAll(t, d, corrected)
	requireBalanced(t, after)
	require.Equal(t, []uuid.UUID{item.BatchIDs[0], received[0].BatchID}, after.Items[0].BatchIDs)
	require.Equal(t, int64(2), after.Items[0].QuarantinedQuantity)
	require.Equal(t, d.Items[1], after.Items[1], "items that are not listed stay unchanged")
}

func TestPlanCorrectionDecreasesAccepted(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item := d.Items[1] // принято 10
	// партии от последней созданной к первой, как их отдаёт запрос
	newer := Batch{ID: uuid.Must(uuid.NewV7()), ProductID: item.ProductID, Quantity: 2}
	older := Batch{ID: uuid.Must(uuid.NewV7()), ProductID: item.ProductID, Quantity: 6}
	adjustment := func(c DeliveryApprovalCorrected, batch Batch, delta int64) StockAdjusted {
		return StockAdjusted{
			ProductID: item.ProductID, BatchID: batch.ID, Delta: delta,
			Reason: "approval_correction", CorrectionID: &c.CorrectionID, AdjustedBy: manager,
		}
	}

	tests := []struct {
		name        string
		batches     []Batch
		accepted    int64
		wantDeltas  []int64 // по партиям newer, older; 0 - партия не тронута
		wantInvalid bool
	}{
		{"from one batch", []Batch{older}, 7, []int64{0, -3}, false},
		{"from two batches, the newest first", []Batch{newer, older}, 5, []int64{-2, -3}, false},
		{"exactly the remaining stock", []Batch{newer, older}, 2, []int64{-2, -6}, false},
		{"more than the remaining stock", []Batch{newer, older}, 1, nil, true},
		{"everything is consumed", nil, 9, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrected, received, adjusted, err := d.planCorrection(ApprovalCorrection{
				Reason: "ошибка пересчёта",
				Items:  []ItemCorrection{{ItemID: item.ID, AcceptedQuantity: tt.accepted, RejectedQuantity: 10 - tt.accepted}},
			}, map[uuid.UUID][]Batch{item.ID: tt.batches}, manager, now)

			if tt.wantInvalid {
				requireInvalid(t, err)
				require.Empty(t, adjusted)
				return
			}
			require.NoError(t, err)
			require.Empty(t, received)
			var want []StockAdjusted
			for i, batch := range []Batch{newer, older} {
				if tt.wantDeltas[i] != 0 {
					want = append(want, adjustment(corrected, batch, tt.wantDeltas[i]))
				}
			}
			require.Equal(t, want, adjusted)
			require.Nil(t, corrected.Items[0].BatchID)
			requireBalanced(t, applyAll(t, d, corrected))
		})
	}
}

func TestPlanCorrectionWithoutStockChange(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item := d.Items[0] // принято 6, карантин 4

	corrected, received, adjusted, err := d.planCorrection(ApprovalCorrection{
		Reason: "часть карантина сразу в брак",
		Items:  []ItemCorrection{{ItemID: item.ID, AcceptedQuantity: 6, RejectedQuantity: 3, QuarantinedQuantity: 1}},
	}, nil, manager, now)

	require.NoError(t, err)
	require.Empty(t, received)
	require.Empty(t, adjusted)
	after := applyAll(t, d, corrected)
	requireBalanced(t, after)
	require.Equal(t, []int64{6, 3, 1},
		[]int64{after.Items[0].AcceptedQuantity, after.Items[0].RejectedQuantity, after.Items[0].QuarantinedQuantity})
}

func TestPlanCorrectionRejected(t *testing.T) {
	d := approvedDelivery(t, fresh)
	first := d.Items[0] // принято 6, карантин 4
	ok := ItemCorrection{ItemID: first.ID, AcceptedQuantity: 8, QuarantinedQuantity: 2}

	t.Run("delivery is not approved", func(t *testing.T) {
		pending := registered(t, fresh)
		_, _, _, err := pending.planCorrection(ApprovalCorrection{Reason: "r", Items: []ItemCorrection{ok}}, nil, manager, now)
		require.ErrorIs(t, err, ErrNotApproved)
	})

	invalid := map[string]ApprovalCorrection{
		"no reason":         {Reason: "  ", Items: []ItemCorrection{ok}},
		"no items":          {Reason: "r"},
		"unknown item":      {Reason: "r", Items: []ItemCorrection{{ItemID: uuid.Must(uuid.NewV7()), AcceptedQuantity: 1}}},
		"item twice":        {Reason: "r", Items: []ItemCorrection{ok, ok}},
		"sum is changed":    {Reason: "r", Items: []ItemCorrection{{ItemID: first.ID, AcceptedQuantity: 8, RejectedQuantity: 3}}},
		"negative quantity": {Reason: "r", Items: []ItemCorrection{{ItemID: first.ID, AcceptedQuantity: 11, RejectedQuantity: -1}}},
		// сумма переполняется и совпадает с заявленным количеством 10
		"sum overflows": {Reason: "r", Items: []ItemCorrection{{
			ItemID: first.ID, AcceptedQuantity: math.MaxInt64, RejectedQuantity: math.MaxInt64, QuarantinedQuantity: 12,
		}}},
		"nothing changes": {Reason: "r", Items: []ItemCorrection{{ItemID: first.ID, AcceptedQuantity: 6, QuarantinedQuantity: 4}}},
	}
	for name, in := range invalid {
		t.Run(name, func(t *testing.T) {
			_, received, adjusted, err := d.planCorrection(in, nil, manager, now)
			requireInvalid(t, err)
			require.Empty(t, received)
			require.Empty(t, adjusted)
		})
	}

	t.Run("expired item cannot be accepted more", func(t *testing.T) {
		afterExpiry := first.ExpiresAt.Add(time.Minute)

		_, _, _, err := d.planCorrection(ApprovalCorrection{Reason: "r", Items: []ItemCorrection{ok}}, nil, manager, afterExpiry)
		requireInvalid(t, err)

		// уменьшить принятое у просроченной позиции можно
		batch := Batch{ID: uuid.Must(uuid.NewV7()), ProductID: first.ProductID, Quantity: 6}
		_, _, adjusted, err := d.planCorrection(ApprovalCorrection{
			Reason: "r", Items: []ItemCorrection{{ItemID: first.ID, AcceptedQuantity: 4, RejectedQuantity: 2, QuarantinedQuantity: 4}},
		}, map[uuid.UUID][]Batch{first.ID: {batch}}, manager, afterExpiry)
		require.NoError(t, err)
		require.Len(t, adjusted, 1)
	})
}

func TestFoldDeliveryWithAllEvents(t *testing.T) {
	d := approvedDelivery(t, fresh)
	item := d.Items[0]

	resolution, batch, err := d.planQuarantineResolution(
		QuarantineDecision{ItemID: item.ID, AcceptedQuantity: 3, RejectedQuantity: 1, RejectReason: "damaged", Comment: "c"}, manager, now)
	require.NoError(t, err)
	d = applyAll(t, d, resolution...)

	corrected, received, _, err := d.planCorrection(ApprovalCorrection{
		Reason: "нашли ещё одну годную", Items: []ItemCorrection{{ItemID: item.ID, AcceptedQuantity: 10}},
	}, nil, manager, now)
	require.NoError(t, err)
	d = applyAll(t, d, corrected)

	requireBalanced(t, d)
	require.Equal(t, int64(10), d.Items[0].AcceptedQuantity)
	require.Equal(t, []uuid.UUID{item.BatchIDs[0], batch.BatchID, received[0].BatchID}, d.Items[0].BatchIDs,
		"batches from approval, quarantine and correction")
}
