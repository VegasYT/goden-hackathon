package inventory

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
)

// GetStock реализует GET /stock.
func (h *HTTP) GetStock(ctx context.Context, params oas.GetStockParams) (*oas.StockPage, error) {
	// значения по умолчанию подставляет сгенерированный разбор запроса
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	// у сортировки минус перед полем - обратный порядок
	field, descending := strings.CutPrefix(string(params.Sort.Value), "-")
	levels, total, err := h.svc.StockView(ctx, point,
		StockQuery{
			NamePrefix: params.Q.Or(""), Category: params.Category.Or(""), Status: StockStatus(params.Status.Or("")),
			Sort: StockSort(field), Descending: descending,
		},
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.StockPage{Items: make([]oas.StockLevel, len(levels)), Total: total}
	for i, level := range levels {
		item := oas.StockLevel{
			Product:           *catalog.ProductResponse(level.Product),
			AvailableQuantity: level.Available, ExpiredQuantity: level.Expired, Batches: level.Batches,
		}
		if level.NearestExpiresAt != nil {
			item.NearestExpiresAt = oas.NewOptDateTime(*level.NearestExpiresAt)
		}
		page.Items[i] = item
	}
	return page, nil
}

// GetStockSummary реализует GET /stock/summary.
func (h *HTTP) GetStockSummary(ctx context.Context, params oas.GetStockSummaryParams) (*oas.StockSummary, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	summary, err := h.svc.StockSummary(ctx, point, StockQuery{NamePrefix: params.Q.Or(""), Category: params.Category.Or("")})
	if err != nil {
		return nil, err
	}
	return &oas.StockSummary{
		Total: summary.Total, InStock: summary.ByStatus[StockInStock], Expiring: summary.ByStatus[StockExpiring],
		Expired: summary.ByStatus[StockExpired], OutOfStock: summary.ByStatus[StockOutOfStock],
	}, nil
}

// GetDeliveryHistory реализует GET /deliveries/{deliveryId}/history.
func (h *HTTP) GetDeliveryHistory(ctx context.Context, params oas.GetDeliveryHistoryParams) (*oas.DeliveryHistory, error) {
	if _, err := h.svc.DeliveryAt(ctx, params.PointId, params.DeliveryId); err != nil {
		return nil, err
	}
	entries, err := h.svc.DeliveryHistory(ctx, params.DeliveryId)
	if err != nil {
		return nil, err
	}
	history := &oas.DeliveryHistory{Items: make([]oas.DeliveryHistoryEntry, len(entries))}
	for i, entry := range entries {
		item := oas.DeliveryHistoryEntry{
			Type: oas.DeliveryHistoryEntryType(entry.Type), OccurredAt: entry.OccurredAt,
			Actor:    actorResponse(entry.Actor),
			Delivery: *deliveryResponse(entry.Delivery),
		}
		if entry.Reason != "" {
			item.Reason = oas.NewOptString(entry.Reason)
		}
		if entry.RejectReason != "" {
			item.RejectReason = oas.NewOptString(entry.RejectReason)
		}
		if entry.Comment != "" {
			item.Comment = oas.NewOptString(entry.Comment)
		}
		history.Items[i] = item
	}
	named := make([]*oas.Delivery, len(history.Items))
	actors := make([]*oas.Actor, len(history.Items))
	for i := range history.Items {
		named[i], actors[i] = &history.Items[i].Delivery, &history.Items[i].Actor
	}
	if err := h.named(ctx, named...); err != nil {
		return nil, err
	}
	return history, h.withNames(ctx, actors)
}

// ListProductMovements реализует GET /products/{productId}/movements.
func (h *HTTP) ListProductMovements(ctx context.Context, params oas.ListProductMovementsParams) (*oas.MovementPage, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	movements, total, err := h.svc.Movements(ctx, point, params.ProductId,
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.MovementPage{Items: make([]oas.Movement, len(movements)), Total: total}
	for i, m := range movements {
		item := oas.Movement{
			ID: m.ID, Type: oas.MovementType(m.Type), OccurredAt: m.OccurredAt,
			Actor:   actorResponse(m.Actor),
			Changes: make([]oas.BatchChange, len(m.Changes)),
		}
		for j, change := range m.Changes {
			item.Changes[j] = oas.BatchChange{BatchID: change.BatchID, Delta: change.Delta}
		}
		if m.OrderID != nil {
			item.OrderID = oas.NewOptUUID(*m.OrderID)
		}
		if m.DeliveryID != nil {
			item.DeliveryID = oas.NewOptUUID(*m.DeliveryID)
		}
		if m.Reason != "" {
			item.Reason = oas.NewOptString(m.Reason)
		}
		if m.Comment != "" {
			item.Comment = oas.NewOptString(m.Comment)
		}
		if m.ExpiresAt != nil {
			item.ExpiresAt = oas.NewOptDateTime(*m.ExpiresAt)
		}
		page.Items[i] = item
	}
	actors := make([]*oas.Actor, len(page.Items))
	var changes []*oas.BatchChange
	for i := range page.Items {
		actors[i] = &page.Items[i].Actor
		for j := range page.Items[i].Changes {
			changes = append(changes, &page.Items[i].Changes[j])
		}
	}
	if err := h.numberedChanges(ctx, changes); err != nil {
		return nil, err
	}
	return page, h.withNames(ctx, actors)
}

// numberedChanges подставляет изменениям номер поставки партии. Движение знает только партию;
// её поставка берётся из таблицы партий, где остаются и пустые.
func (h *HTTP) numberedChanges(ctx context.Context, changes []*oas.BatchChange) error {
	batchIDs := make([]uuid.UUID, len(changes))
	for i, change := range changes {
		batchIDs[i] = change.BatchID
	}
	deliveries, err := h.svc.BatchDeliveries(ctx, batchIDs)
	if err != nil {
		return err
	}
	deliveryIDs := make([]uuid.UUID, 0, len(deliveries))
	for _, id := range deliveries {
		deliveryIDs = append(deliveryIDs, id)
	}
	numbers, err := h.svc.DeliveryNumbers(ctx, deliveryIDs)
	if err != nil {
		return err
	}
	for _, change := range changes {
		change.DeliveryNumber = numbers[deliveries[change.BatchID]]
	}
	return nil
}

// ListExpiringBatches реализует GET /batches/expiring.
func (h *HTTP) ListExpiringBatches(ctx context.Context, params oas.ListExpiringBatchesParams) (*oas.ExpiringBatchPage, error) {
	// значения по умолчанию и границы горизонта проверяет сгенерированный разбор запроса
	batches, total, err := h.svc.ExpiringBatches(ctx, params.PointId, time.Duration(params.WithinHours.Value)*time.Hour,
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.ExpiringBatchPage{Items: make([]oas.ExpiringBatch, len(batches)), Total: total}
	numbered := make([]*oas.Batch, len(batches))
	for i, b := range batches {
		page.Items[i] = oas.ExpiringBatch{
			Batch: *batchResponse(b.Batch), Product: *catalog.ProductResponse(b.Product), Expired: b.Expired,
		}
		numbered[i] = &page.Items[i].Batch
	}
	return page, h.numbered(ctx, numbered...)
}
