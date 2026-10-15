package inventory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

// Карантин, его решения и исходное распределение собирает свёртка из событий потока.
func TestFoldDeliveryQuarantine(t *testing.T) {
	d := registered(t, fresh)
	itemID := d.Items[0].ID
	approved, _, err := d.planApproval([]ItemDecision{
		{
			ItemID: itemID, AcceptedQuantity: 4, RejectedQuantity: 2, QuarantinedQuantity: 4,
			RejectReason: "physical_damage", QuarantineReason: "storage_violation", Comment: "c",
		},
		acceptAll(d.Items[1]),
	}, manager, now)
	require.NoError(t, err)
	d, err = d.apply(approved)
	require.NoError(t, err)

	atApproval := Distribution{AcceptedQuantity: 4, RejectedQuantity: 2, QuarantinedQuantity: 4}
	require.Equal(t, &atApproval, d.Items[0].Approved)
	require.Equal(t, now, *d.Items[0].QuarantinedAt)
	require.Nil(t, d.Items[1].QuarantinedAt, "an item without quarantine has no quarantine time")
	require.Nil(t, d.CorrectedAt)

	// одно решение - два события с одним временем и одна запись у позиции
	decidedAt := now.Add(time.Hour)
	events, _, err := d.planQuarantineResolution(QuarantineDecision{
		ItemID: itemID, AcceptedQuantity: 3, RejectedQuantity: 1, RejectReason: "quality_issue", Comment: "одна вздулась",
	}, manager, decidedAt)
	require.NoError(t, err)
	require.Len(t, events, 2)
	before := d
	for _, event := range events {
		d, err = d.apply(event)
		require.NoError(t, err)
	}
	require.Equal(t, []QuarantineResolved{{
		DecidedAt: decidedAt, DecidedBy: manager, AcceptedQuantity: 3, RejectedQuantity: 1,
		RejectReason: "quality_issue", Comment: "одна вздулась",
	}}, d.Items[0].Resolutions)
	require.Nil(t, d.Items[0].QuarantinedAt, "a resolved quarantine is closed")
	require.Empty(t, before.Items[0].Resolutions, "apply must not change the previous state")

	// исправление открывает карантин заново и не трогает распределение приёмки
	correct := func(at time.Time, quarantined int64) {
		d, err = d.apply(DeliveryApprovalCorrected{
			DeliveryID: d.ID, CorrectedAt: eventstore.At(at),
			Items: []CorrectedItem{{ItemID: itemID, AcceptedQuantity: 7 - quarantined, RejectedQuantity: 3, QuarantinedQuantity: quarantined}},
		})
		require.NoError(t, err)
	}
	reopenedAt := now.Add(2 * time.Hour)
	correct(reopenedAt, 2)
	require.Equal(t, reopenedAt, *d.Items[0].QuarantinedAt)
	require.Equal(t, reopenedAt, *d.CorrectedAt)

	correct(now.Add(3*time.Hour), 1)
	require.Equal(t, reopenedAt, *d.Items[0].QuarantinedAt, "the quarantine was already open")
	require.Equal(t, now.Add(3*time.Hour), *d.CorrectedAt)

	correct(now.Add(4*time.Hour), 0)
	require.Nil(t, d.Items[0].QuarantinedAt)
	require.Equal(t, &atApproval, d.Items[0].Approved)
	require.Len(t, d.Items[0].Resolutions, 1)
}
