package inventory

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

// correctionReason - причина события StockAdjusted, которым корректировка приёмки снимает количество с партии.
const correctionReason = "approval_correction"

// QuarantineDecision - решение по карантину позиции. Всё карантинное количество
// делится на принятую и отклонённую части; любая может быть нулевой (ADR-010).
type QuarantineDecision struct {
	ItemID           uuid.UUID
	AcceptedQuantity int64
	RejectedQuantity int64
	// причина и комментарий обязательны для отклонённой части
	RejectReason string
	Comment      string
}

// ApprovalCorrection - корректировка приёмки: новое распределение количества по позициям.
type ApprovalCorrection struct {
	Reason string
	Items  []ItemCorrection
}

// ItemCorrection - итоговое распределение заявленного количества позиции после корректировки.
type ItemCorrection struct {
	ItemID              uuid.UUID
	AcceptedQuantity    int64
	RejectedQuantity    int64
	QuarantinedQuantity int64
}

// planQuarantineResolution проверяет решение по карантину и формирует события потока поставки
// и, для принятой части, событие новой партии.
func (d Delivery) planQuarantineResolution(
	in QuarantineDecision, actor eventstore.Actor, now time.Time,
) ([]eventstore.Payload, *BatchReceived, error) {
	if d.Status != DeliveryApprovedStatus {
		return nil, nil, fmt.Errorf("delivery %s: %w", d.ID, ErrNotApproved)
	}
	item, found := d.item(in.ItemID)
	if !found {
		return nil, nil, invalid("item_id", fmt.Sprintf("item %s is not part of the delivery", in.ItemID))
	}
	if item.QuarantinedQuantity == 0 {
		return nil, nil, fmt.Errorf("item %s: %w", item.ID, ErrQuarantineClosed)
	}

	accepted, rejected := in.AcceptedQuantity, in.RejectedQuantity
	reason, comment := strings.TrimSpace(in.RejectReason), strings.TrimSpace(in.Comment)
	switch {
	case accepted < 0 || rejected < 0:
		return nil, nil, invalid("quantity", "quantities must not be negative")
	case accepted+rejected != item.QuarantinedQuantity:
		return nil, nil, invalid("quantity",
			fmt.Sprintf("accepted + rejected must equal the quarantined quantity %d", item.QuarantinedQuantity))
	case rejected > 0 && reason == "":
		return nil, nil, invalid("reject_reason", "is required for rejected quantity")
	case rejected > 0 && comment == "":
		return nil, nil, invalid("comment", "is required for rejected quantity")
	case accepted > 0 && !item.ExpiresAt.After(now):
		return nil, nil, expiredField("accepted_quantity", "item is expired and cannot be accepted")
	}

	var (
		events []eventstore.Payload
		batch  *BatchReceived
	)
	if accepted > 0 {
		batchID := uuid.Must(uuid.NewV7())
		events = append(events, QuarantinedDeliveryItemAccepted{
			PointID: d.PointID, DeliveryID: d.ID, ItemID: item.ID, Quantity: accepted, BatchID: batchID,
			Comment: comment, DecidedAt: eventstore.At(now), DecidedBy: actor,
		})
		batch = &BatchReceived{
			PointID: d.PointID, BatchID: batchID, ProductID: item.ProductID, DeliveryID: d.ID, ItemID: item.ID,
			Quantity: accepted, ExpiresAt: eventstore.At(item.ExpiresAt),
			ReceivedAt: eventstore.At(now), Source: BatchFromQuarantine,
		}
	}
	if rejected > 0 {
		events = append(events, QuarantinedDeliveryItemRejected{
			PointID: d.PointID, DeliveryID: d.ID, ItemID: item.ID, Quantity: rejected, Reason: reason,
			Comment: comment, DecidedAt: eventstore.At(now), DecidedBy: actor,
		})
	}
	return events, batch, nil
}

// planCorrection проверяет корректировку приёмки и формирует событие корректировки,
// новые партии для увеличения принятого количества и корректировки остатков для уменьшения.
// batches - текущие партии исправляемых позиций, от последней созданной к первой.
func (d Delivery) planCorrection(
	in ApprovalCorrection, batches map[uuid.UUID][]Batch, actor eventstore.Actor, now time.Time,
) (DeliveryApprovalCorrected, []BatchReceived, []StockAdjusted, error) {
	fail := func(err error) (DeliveryApprovalCorrected, []BatchReceived, []StockAdjusted, error) {
		return DeliveryApprovalCorrected{}, nil, nil, err
	}
	reason := strings.TrimSpace(in.Reason)
	switch {
	case d.Status != DeliveryApprovedStatus:
		return fail(fmt.Errorf("delivery %s: %w", d.ID, ErrNotApproved))
	case reason == "":
		return fail(invalid("reason", "is required"))
	case len(in.Items) == 0:
		return fail(invalid("items", "must not be empty"))
	}

	corrected := DeliveryApprovalCorrected{
		PointID: d.PointID, CorrectionID: uuid.Must(uuid.NewV7()), DeliveryID: d.ID, Reason: reason,
		CorrectedAt: eventstore.At(now), CorrectedBy: actor,
	}
	var (
		received []BatchReceived
		adjusted []StockAdjusted
	)
	seen := make(map[uuid.UUID]bool, len(in.Items))
	for _, c := range in.Items {
		field := func(name string) string { return fmt.Sprintf("items[%s].%s", c.ItemID, name) }
		item, found := d.item(c.ItemID)
		switch {
		case !found:
			return fail(invalid("items", fmt.Sprintf("item %s is not part of the delivery", c.ItemID)))
		case seen[c.ItemID]:
			return fail(invalid("items", fmt.Sprintf("item %s is corrected twice", c.ItemID)))
		case c.AcceptedQuantity < 0 || c.RejectedQuantity < 0 || c.QuarantinedQuantity < 0:
			return fail(invalid(field("quantity"), "quantities must not be negative"))
		// каждая часть не больше заявленного: иначе сумма трёх int64 переполняется и совпадает с ним
		case c.AcceptedQuantity > item.DeclaredQuantity || c.RejectedQuantity > item.DeclaredQuantity ||
			c.QuarantinedQuantity > item.DeclaredQuantity:
			return fail(invalid(field("quantity"),
				fmt.Sprintf("each quantity must not exceed declared quantity %d", item.DeclaredQuantity)))
		case c.AcceptedQuantity+c.RejectedQuantity+c.QuarantinedQuantity != item.DeclaredQuantity:
			return fail(invalid(field("quantity"),
				fmt.Sprintf("accepted + rejected + quarantined must equal declared quantity %d", item.DeclaredQuantity)))
		}
		seen[c.ItemID] = true

		if c.AcceptedQuantity == item.AcceptedQuantity && c.RejectedQuantity == item.RejectedQuantity &&
			c.QuarantinedQuantity == item.QuarantinedQuantity {
			continue
		}

		out := CorrectedItem{
			ItemID: item.ID, AcceptedQuantity: c.AcceptedQuantity,
			RejectedQuantity: c.RejectedQuantity, QuarantinedQuantity: c.QuarantinedQuantity,
		}
		switch delta := c.AcceptedQuantity - item.AcceptedQuantity; {
		case delta > 0:
			if !item.ExpiresAt.After(now) {
				return fail(expiredField(field("accepted_quantity"), "item is expired, accepted quantity cannot grow"))
			}
			batchID := uuid.Must(uuid.NewV7())
			out.BatchID = &batchID
			received = append(received, BatchReceived{
				PointID: d.PointID, BatchID: batchID, ProductID: item.ProductID, DeliveryID: d.ID, ItemID: item.ID,
				Quantity: delta, ExpiresAt: eventstore.At(item.ExpiresAt),
				ReceivedAt: eventstore.At(now), Source: BatchFromCorrection,
			})
		case delta < 0:
			// снять можно только то, что ещё лежит на складе: израсходованное не вернуть
			left := -delta
			for _, batch := range batches[item.ID] {
				take := min(batch.Quantity, left)
				if take == 0 {
					continue
				}
				adjusted = append(adjusted, StockAdjusted{
					PointID: d.PointID, ProductID: item.ProductID, BatchID: batch.ID, Delta: -take,
					Reason: correctionReason, CorrectionID: &corrected.CorrectionID, AdjustedBy: actor,
				})
				left -= take
			}
			if left > 0 {
				limit := -delta - left
				return fail(&ValidationError{
					Field: field("accepted_quantity"), Code: CodeStockConsumed, Limit: &limit,
					Message: fmt.Sprintf("can be reduced by at most %d: the rest is already consumed", limit),
				})
			}
		}
		corrected.Items = append(corrected.Items, out)
	}

	if len(corrected.Items) == 0 {
		return fail(invalid("items", "correction changes nothing"))
	}
	return corrected, received, adjusted, nil
}
