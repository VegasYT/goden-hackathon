//go:build integration

package inventory_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/eventstore"
)

// approve регистрирует поставку из одной позиции и принимает её с заданным распределением.
func (f fixture) approve(
	t *testing.T, product catalog.Product, expiresAt time.Time, accepted, rejected, quarantined int64,
) inventory.Delivery {
	t.Helper()
	registered := f.register(t, item(product, accepted+rejected+quarantined, expiresAt))
	decision := inventory.ItemDecision{
		ItemID: registered.Items[0].ID, AcceptedQuantity: accepted,
		RejectedQuantity: rejected, QuarantinedQuantity: quarantined,
	}
	if rejected > 0 {
		decision.RejectReason, decision.Comment = "damaged_package", "повреждена упаковка"
	}
	if quarantined > 0 {
		decision.QuarantineReason, decision.Comment = "temperature_check", "проверить температуру"
	}
	approved, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, []inventory.ItemDecision{decision})
	require.NoError(t, err)
	return approved
}

func distribution(it inventory.DeliveryItemState) []int64 {
	return []int64{it.AcceptedQuantity, it.RejectedQuantity, it.QuarantinedQuantity}
}

func eventTypes(events []eventstore.Payload) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.EventType()
	}
	return types
}

// Решение по карантину.

func TestResolveQuarantine(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 0, 4)
	itemID := approved.Items[0].ID
	f.clock.Advance(2 * time.Hour)
	decidedAt := f.clock.Now()

	resolved, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{
		ItemID: itemID, AcceptedQuantity: 3, RejectedQuantity: 1, RejectReason: "damaged", Comment: "одна вздулась",
	})
	require.NoError(t, err)

	// состояние поставки
	got := resolved.Items[0]
	require.Equal(t, []int64{9, 1, 0}, distribution(got))
	require.Len(t, got.BatchIDs, 2)
	read, err := f.svc.Delivery(f.ctx, approved.ID)
	require.NoError(t, err)
	require.Equal(t, resolved, read)

	// новая партия на принятую часть
	newBatch := f.batch(t, got.BatchIDs[1])
	require.Equal(t, inventory.Batch{
		PointID: point.ID, ID: got.BatchIDs[1], ProductID: cream.ID, DeliveryID: approved.ID, ItemID: itemID,
		ReceivedQuantity: 3, Quantity: 3, ExpiresAt: oct(15), ReceivedAt: decidedAt,
	}, newBatch)
	require.Equal(t, inventory.BatchReceived{
		PointID: point.ID, BatchID: newBatch.ID, ProductID: cream.ID, DeliveryID: approved.ID, ItemID: itemID,
		Quantity: 3, ExpiresAt: eventstore.At(oct(15)), ReceivedAt: eventstore.At(decidedAt),
		Source: inventory.BatchFromQuarantine,
	}, f.lastStockEvent(t, cream))

	// события поставки
	events := f.events(t, inventory.DeliveryStream(approved.ID))
	require.Equal(t, []string{
		"DeliveryRegistered", "DeliveryApproved", "QuarantinedDeliveryItemAccepted", "QuarantinedDeliveryItemRejected",
	}, eventTypes(events))
	require.Equal(t, inventory.QuarantinedDeliveryItemAccepted{
		PointID: approved.PointID, DeliveryID: approved.ID, ItemID: itemID, Quantity: 3, BatchID: newBatch.ID,
		Comment: "одна вздулась", DecidedAt: eventstore.At(decidedAt), DecidedBy: manager,
	}, events[2])
	require.Equal(t, inventory.QuarantinedDeliveryItemRejected{
		PointID: approved.PointID, DeliveryID: approved.ID, ItemID: itemID, Quantity: 1, Reason: "damaged",
		Comment: "одна вздулась", DecidedAt: eventstore.At(decidedAt), DecidedBy: manager,
	}, events[3])

	// партии с ранним сроком, включая партию из карантина, расходуются раньше поздней
	f.supply(t, cream, 100, oct(20))
	shortages, err := f.consume(t, []catalog.Product{cream}, inventory.Need{ProductID: cream.ID, Quantity: 9})
	require.NoError(t, err)
	require.Empty(t, shortages)
	require.Equal(t, []int64{0, 0, 100}, f.quantities(t, cream))
}

func TestResolveQuarantineWholeQuantity(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)

	t.Run("accept", func(t *testing.T) {
		approved := f.approve(t, cream, oct(15), 6, 0, 4)

		resolved, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID,
			inventory.QuarantineDecision{ItemID: approved.Items[0].ID, AcceptedQuantity: 4})

		require.NoError(t, err)
		require.Equal(t, []int64{10, 0, 0}, distribution(resolved.Items[0]))
		require.Equal(t, int64(4), f.batch(t, resolved.Items[0].BatchIDs[1]).Quantity)
	})

	t.Run("reject", func(t *testing.T) {
		approved := f.approve(t, cream, oct(15), 6, 0, 4)
		stockEvents := len(f.events(t, inventory.StockStream(point.ID, cream.ID)))

		resolved, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{
			ItemID: approved.Items[0].ID, RejectedQuantity: 4, RejectReason: "temperature_violation", Comment: "выше нормы",
		})

		require.NoError(t, err)
		require.Equal(t, []int64{6, 4, 0}, distribution(resolved.Items[0]))
		require.Len(t, resolved.Items[0].BatchIDs, 1, "no new batch")
		require.Len(t, f.events(t, inventory.StockStream(point.ID, cream.ID)), stockEvents)
	})
}

func TestResolveQuarantineRejected(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 0, 4)
	itemID := approved.Items[0].ID
	pending := f.register(t, item(cream, 10, oct(15)))
	before := f.totalEvents(t)

	_, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID,
		inventory.QuarantineDecision{ItemID: itemID, AcceptedQuantity: 2, RejectedQuantity: 1, RejectReason: "r", Comment: "c"})
	requireInvalid(t, err)

	_, err = f.svc.ResolveQuarantine(f.ctx, manager, "", pending.ID,
		inventory.QuarantineDecision{ItemID: pending.Items[0].ID, AcceptedQuantity: 1})
	require.ErrorIs(t, err, inventory.ErrNotApproved)

	_, err = f.svc.ResolveQuarantine(f.ctx, manager, "", uuid.Must(uuid.NewV7()),
		inventory.QuarantineDecision{ItemID: itemID, AcceptedQuantity: 4})
	require.ErrorIs(t, err, inventory.ErrNotFound)

	require.Equal(t, before, f.totalEvents(t))
	require.Equal(t, []int64{6}, f.quantities(t, cream))

	// после решения карантин закрыт
	_, err = f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{ItemID: itemID, AcceptedQuantity: 4})
	require.NoError(t, err)
	before = f.totalEvents(t)

	_, err = f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{ItemID: itemID, AcceptedQuantity: 4})
	require.ErrorIs(t, err, inventory.ErrQuarantineClosed)
	require.Equal(t, before, f.totalEvents(t))
}

func TestResolveQuarantineOfItemExpiredInQuarantine(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(12), 6, 0, 4)
	itemID := approved.Items[0].ID
	f.clock.Set(oct(13))

	_, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{ItemID: itemID, AcceptedQuantity: 4})
	requireInvalid(t, err)

	resolved, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{
		ItemID: itemID, RejectedQuantity: 4, RejectReason: "expired", Comment: "срок истёк в карантине",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{6, 4, 0}, distribution(resolved.Items[0]))
}

func TestConcurrentQuarantineResolutions(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 0, 4)
	decision := inventory.QuarantineDecision{ItemID: approved.Items[0].ID, AcceptedQuantity: 4}

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, decision)
			errs <- err
		}()
	}
	first, second := <-errs, <-errs

	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, inventory.ErrQuarantineClosed)
	require.Equal(t, []int64{6, 4}, f.quantities(t, cream), "the quarantine batch is created once")
}

func TestResolveQuarantineIsIdempotent(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 0, 4)
	decision := inventory.QuarantineDecision{ItemID: approved.Items[0].ID, AcceptedQuantity: 4}

	first, err := f.svc.ResolveQuarantine(f.ctx, manager, "key-1", approved.ID, decision)
	require.NoError(t, err)
	before := f.totalEvents(t)
	second, err := f.svc.ResolveQuarantine(f.ctx, manager, "key-1", approved.ID, decision)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, before, f.totalEvents(t))
}

// Корректировка приёмки.

func correct(itemID uuid.UUID, accepted, rejected, quarantined int64) inventory.ApprovalCorrection {
	return inventory.ApprovalCorrection{Reason: "ошибка пересчёта", Items: []inventory.ItemCorrection{{
		ItemID: itemID, AcceptedQuantity: accepted, RejectedQuantity: rejected, QuarantinedQuantity: quarantined,
	}}}
}

func TestCorrectDeliveryApprovalIncreasesAccepted(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 4, 0)
	itemID := approved.Items[0].ID
	original := f.events(t, inventory.DeliveryStream(approved.ID))[1]
	f.clock.Advance(time.Hour)
	correctedAt := f.clock.Now()

	corrected, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 8, 2, 0))
	require.NoError(t, err)

	got := corrected.Items[0]
	require.Equal(t, []int64{8, 2, 0}, distribution(got))
	require.Len(t, got.BatchIDs, 2)
	require.Equal(t, []int64{6, 2}, f.quantities(t, cream))
	require.Equal(t, inventory.BatchReceived{
		PointID: point.ID, BatchID: got.BatchIDs[1], ProductID: cream.ID, DeliveryID: approved.ID, ItemID: itemID,
		Quantity: 2, ExpiresAt: eventstore.At(oct(15)), ReceivedAt: eventstore.At(correctedAt),
		Source: inventory.BatchFromCorrection,
	}, f.lastStockEvent(t, cream))

	events := f.events(t, inventory.DeliveryStream(approved.ID))
	require.Equal(t, []string{"DeliveryRegistered", "DeliveryApproved", "DeliveryApprovalCorrected"}, eventTypes(events))
	require.Equal(t, original, events[1], "the original approval is not changed")
	event := events[2].(inventory.DeliveryApprovalCorrected)
	require.Equal(t, inventory.DeliveryApprovalCorrected{
		PointID: approved.PointID, CorrectionID: event.CorrectionID, DeliveryID: approved.ID, Reason: "ошибка пересчёта",
		CorrectedAt: eventstore.At(correctedAt), CorrectedBy: manager,
		Items: []inventory.CorrectedItem{{ItemID: itemID, AcceptedQuantity: 8, RejectedQuantity: 2, BatchID: &got.BatchIDs[1]}},
	}, event)
	require.NotEqual(t, uuid.Nil, event.CorrectionID)

	read, err := f.svc.Delivery(f.ctx, approved.ID)
	require.NoError(t, err)
	require.Equal(t, corrected, read)
}

func TestCorrectDeliveryApprovalDecreasesAccepted(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 10, 0, 0)
	itemID, batchID := approved.Items[0].ID, approved.Items[0].BatchIDs[0]

	corrected, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 7, 3, 0))
	require.NoError(t, err)

	require.Equal(t, []int64{7, 3, 0}, distribution(corrected.Items[0]))
	require.Len(t, corrected.Items[0].BatchIDs, 1)
	require.Equal(t, int64(7), f.batch(t, batchID).Quantity)
	events := f.events(t, inventory.DeliveryStream(approved.ID))
	correctionID := events[2].(inventory.DeliveryApprovalCorrected).CorrectionID
	require.Equal(t, inventory.StockAdjusted{
		PointID: point.ID, ProductID: cream.ID, BatchID: batchID, Delta: -3,
		Reason: "approval_correction", CorrectionID: &correctionID, AdjustedBy: manager,
	}, f.lastStockEvent(t, cream))
}

func TestCorrectDeliveryApprovalCannotGoBelowConsumed(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 10, 0, 0)
	itemID, batchID := approved.Items[0].ID, approved.Items[0].BatchIDs[0]
	_, err := f.consume(t, []catalog.Product{cream}, inventory.Need{ProductID: cream.ID, Quantity: 7})
	require.NoError(t, err)
	before := f.totalEvents(t)

	// принято 10, израсходовано 7: снять можно не больше 3
	_, err = f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 2, 8, 0))
	var validation *inventory.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Contains(t, validation.Message, "at most 3")
	require.Equal(t, before, f.totalEvents(t))
	require.Equal(t, int64(3), f.batch(t, batchID).Quantity)

	corrected, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 7, 3, 0))
	require.NoError(t, err)
	require.Equal(t, []int64{7, 3, 0}, distribution(corrected.Items[0]))
	require.Zero(t, f.batch(t, batchID).Quantity)
}

func TestCorrectDeliveryApprovalTakesFromNewestBatchFirst(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 4, 0)
	itemID := approved.Items[0].ID
	f.clock.Advance(time.Hour)
	grown, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 8, 2, 0))
	require.NoError(t, err)
	fromApproval, fromCorrection := grown.Items[0].BatchIDs[0], grown.Items[0].BatchIDs[1]
	stockEvents := len(f.events(t, inventory.StockStream(point.ID, cream.ID)))

	shrunk, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 3, 7, 0))
	require.NoError(t, err)

	require.Equal(t, []int64{3, 7, 0}, distribution(shrunk.Items[0]))
	require.Zero(t, f.batch(t, fromCorrection).Quantity, "the newest batch is emptied first")
	require.Equal(t, int64(3), f.batch(t, fromApproval).Quantity)
	adjustments := f.events(t, inventory.StockStream(point.ID, cream.ID))[stockEvents:]
	require.Len(t, adjustments, 2)
	require.Equal(t, fromCorrection, adjustments[0].(inventory.StockAdjusted).BatchID)
	require.Equal(t, int64(-2), adjustments[0].(inventory.StockAdjusted).Delta)
	require.Equal(t, fromApproval, adjustments[1].(inventory.StockAdjusted).BatchID)
	require.Equal(t, int64(-3), adjustments[1].(inventory.StockAdjusted).Delta)
}

func TestCorrectDeliveryApprovalRejected(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(12), 6, 4, 0)
	itemID := approved.Items[0].ID
	pending := f.register(t, item(cream, 10, oct(15)))
	before := f.totalEvents(t)

	_, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 6, 4, 0))
	requireInvalid(t, err) // ничего не меняет

	_, err = f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 8, 3, 0))
	requireInvalid(t, err) // сумма изменена

	noReason := correct(itemID, 8, 2, 0)
	noReason.Reason = " "
	_, err = f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, noReason)
	requireInvalid(t, err)

	_, err = f.svc.CorrectDeliveryApproval(f.ctx, manager, "", pending.ID, correct(pending.Items[0].ID, 8, 2, 0))
	require.ErrorIs(t, err, inventory.ErrNotApproved)

	f.clock.Set(oct(13)) // позиция просрочена: допринять нельзя
	_, err = f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 8, 2, 0))
	requireInvalid(t, err)

	require.Equal(t, before, f.totalEvents(t))
	require.Equal(t, []int64{6}, f.quantities(t, cream))
}

func TestCorrectionReopensQuarantineAndDeliveryKeepsAllBatches(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 6, 0, 6) // заявлено 12
	itemID := approved.Items[0].ID

	// решение по карантину: 4 принято, 2 отклонено
	f.clock.Advance(time.Hour)
	_, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{
		ItemID: itemID, AcceptedQuantity: 4, RejectedQuantity: 2, RejectReason: "damaged", Comment: "две вздулись",
	})
	require.NoError(t, err)

	// корректировка: отклонённые 2 оказались годными
	f.clock.Advance(time.Hour)
	corrected, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 12, 0, 0))
	require.NoError(t, err)
	got := corrected.Items[0]
	require.Equal(t, []int64{12, 0, 0}, distribution(got))
	require.Len(t, got.BatchIDs, 3, "batches from approval, quarantine and correction")
	require.Equal(t, []int64{6, 4, 2}, f.quantities(t, cream))

	// корректировка возвращает 2 в карантин: снимается с последней партии
	f.clock.Advance(time.Hour)
	reopened, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "", approved.ID, correct(itemID, 10, 0, 2))
	require.NoError(t, err)
	require.Equal(t, []int64{10, 0, 2}, distribution(reopened.Items[0]))
	require.Equal(t, []int64{6, 4, 0}, f.quantities(t, cream))

	// по открытому заново карантину принимается новое решение
	closed, err := f.svc.ResolveQuarantine(f.ctx, manager, "", approved.ID, inventory.QuarantineDecision{
		ItemID: itemID, RejectedQuantity: 2, RejectReason: "damaged", Comment: "всё-таки брак",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{10, 2, 0}, distribution(closed.Items[0]))

	read, err := f.svc.Delivery(f.ctx, approved.ID)
	require.NoError(t, err)
	require.Equal(t, closed, read)
	require.Equal(t, []string{
		"DeliveryRegistered", "DeliveryApproved",
		"QuarantinedDeliveryItemAccepted", "QuarantinedDeliveryItemRejected",
		"DeliveryApprovalCorrected", "DeliveryApprovalCorrected",
		"QuarantinedDeliveryItemRejected",
	}, eventTypes(f.events(t, inventory.DeliveryStream(approved.ID))))
}

func TestCorrectDeliveryApprovalIsIdempotent(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	approved := f.approve(t, cream, oct(15), 10, 0, 0)
	correction := correct(approved.Items[0].ID, 7, 3, 0)

	first, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "key-1", approved.ID, correction)
	require.NoError(t, err)
	before := f.totalEvents(t)
	second, err := f.svc.CorrectDeliveryApproval(f.ctx, manager, "key-1", approved.ID, correction)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, before, f.totalEvents(t))
	require.Equal(t, []int64{7}, f.quantities(t, cream))
}
