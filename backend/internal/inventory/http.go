package inventory

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
	platformredis "goden/backend/internal/platform/redis"
)

// HTTP реализует операции поставок и склада из api/openapi.yaml. Обработчик - тонкая обёртка:
// разбирает запрос, вызывает сервис, собирает ответ. Правил в нём нет.
//
// Все операции лежат под /points/{pointId}; право вызывающего на точке проверил слой API.
// Обработчик отвечает за остальное: поставка и партия из адреса принадлежат этой точке
// (DeliveryAt, BatchAt), а запись идёт только на работающую точку (writable).
type HTTP struct {
	svc    *Service
	points *organization.Service
	cache  *platformredis.Cache
}

func NewHTTP(svc *Service, points *organization.Service, caches ...*platformredis.Cache) *HTTP {
	var cache *platformredis.Cache
	if len(caches) > 0 {
		cache = caches[0]
	}
	return &HTTP{svc: svc, points: points, cache: cache}
}

// writable возвращает автора и точку для операции записи.
func (h *HTTP) writable(ctx context.Context, pointID uuid.UUID) (eventstore.Actor, organization.Point, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return actor, organization.Point{}, err
	}
	point, err := h.points.WorkingPoint(ctx, pointID)
	return actor, point, err
}

// deliveryWritable - то же для операции над поставкой точки.
func (h *HTTP) deliveryWritable(ctx context.Context, pointID, deliveryID uuid.UUID) (eventstore.Actor, error) {
	actor, _, err := h.writable(ctx, pointID)
	if err != nil {
		return actor, err
	}
	_, err = h.svc.DeliveryAt(ctx, pointID, deliveryID)
	return actor, err
}

// batchWritable - то же для операции над партией точки.
func (h *HTTP) batchWritable(ctx context.Context, pointID, batchID uuid.UUID) (eventstore.Actor, error) {
	actor, _, err := h.writable(ctx, pointID)
	if err != nil {
		return actor, err
	}
	_, err = h.svc.BatchAt(ctx, pointID, batchID)
	return actor, err
}

func actorOf(ctx context.Context) (eventstore.Actor, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return eventstore.Actor{}, errors.New("inventory: request has no actor")
	}
	return actor, nil
}

// Поставки.

// RegisterDelivery реализует POST /deliveries.
func (h *HTTP) RegisterDelivery(ctx context.Context, req *oas.NewDelivery, params oas.RegisterDeliveryParams) (*oas.Delivery, error) {
	actor, point, err := h.writable(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	in := NewDelivery{Number: req.Number, DeliveredAt: req.DeliveredAt, Items: make([]NewDeliveryItem, len(req.Items))}
	for i, item := range req.Items {
		in.Items[i] = NewDeliveryItem{
			ProductID: item.ProductID, DeclaredQuantity: item.DeclaredQuantity, ExpiresAt: item.ExpiresAt,
		}
		if made, set := item.ManufacturedAt.Get(); set {
			in.Items[i].ManufacturedAt = &made
		}
	}

	delivery, err := h.svc.RegisterDelivery(ctx, actor, params.IdempotencyKey.Or(""), point, in)
	if err != nil {
		return nil, err
	}
	return h.deliveryOut(ctx, delivery)
}

// GetDelivery реализует GET /deliveries/{deliveryId}.
func (h *HTTP) GetDelivery(ctx context.Context, params oas.GetDeliveryParams) (*oas.Delivery, error) {
	delivery, err := h.svc.DeliveryAt(ctx, params.PointId, params.DeliveryId)
	if err != nil {
		return nil, err
	}
	return h.deliveryOut(ctx, delivery)
}

// ApproveDelivery реализует POST /deliveries/{deliveryId}/approval.
func (h *HTTP) ApproveDelivery(ctx context.Context, req *oas.DeliveryApproval, params oas.ApproveDeliveryParams) (*oas.Delivery, error) {
	actor, err := h.deliveryWritable(ctx, params.PointId, params.DeliveryId)
	if err != nil {
		return nil, err
	}
	decisions := make([]ItemDecision, len(req.Items))
	for i, item := range req.Items {
		decisions[i] = ItemDecision{
			ItemID: item.ItemID, AcceptedQuantity: item.AcceptedQuantity,
			RejectedQuantity: item.RejectedQuantity, QuarantinedQuantity: item.QuarantinedQuantity,
			RejectReason: string(item.RejectReason.Or("")), QuarantineReason: string(item.QuarantineReason.Or("")),
			Comment: item.Comment.Or(""),
		}
	}

	delivery, err := h.svc.ApproveDelivery(ctx, actor, params.IdempotencyKey.Or(""), params.DeliveryId, decisions)
	if err != nil {
		return nil, err
	}
	return h.deliveryOut(ctx, delivery)
}

// ResolveQuarantine реализует POST /deliveries/{deliveryId}/items/{itemId}/quarantine-resolution.
func (h *HTTP) ResolveQuarantine(
	ctx context.Context, req *oas.QuarantineResolution, params oas.ResolveQuarantineParams,
) (*oas.Delivery, error) {
	actor, err := h.deliveryWritable(ctx, params.PointId, params.DeliveryId)
	if err != nil {
		return nil, err
	}
	delivery, err := h.svc.ResolveQuarantine(ctx, actor, params.IdempotencyKey.Or(""), params.DeliveryId, QuarantineDecision{
		ItemID: params.ItemId, AcceptedQuantity: req.AcceptedQuantity, RejectedQuantity: req.RejectedQuantity,
		RejectReason: string(req.RejectReason.Or("")), Comment: req.Comment.Or(""),
	})
	if err != nil {
		return nil, err
	}
	return h.deliveryOut(ctx, delivery)
}

// CorrectDeliveryApproval реализует POST /deliveries/{deliveryId}/corrections.
func (h *HTTP) CorrectDeliveryApproval(
	ctx context.Context, req *oas.ApprovalCorrection, params oas.CorrectDeliveryApprovalParams,
) (*oas.Delivery, error) {
	actor, err := h.deliveryWritable(ctx, params.PointId, params.DeliveryId)
	if err != nil {
		return nil, err
	}
	in := ApprovalCorrection{Reason: req.Reason, Items: make([]ItemCorrection, len(req.Items))}
	for i, item := range req.Items {
		in.Items[i] = ItemCorrection{
			ItemID: item.ItemID, AcceptedQuantity: item.AcceptedQuantity,
			RejectedQuantity: item.RejectedQuantity, QuarantinedQuantity: item.QuarantinedQuantity,
		}
	}

	delivery, err := h.svc.CorrectDeliveryApproval(ctx, actor, params.IdempotencyKey.Or(""), params.DeliveryId, in)
	if err != nil {
		return nil, err
	}
	return h.deliveryOut(ctx, delivery)
}

// Склад.

// ListProductBatches реализует GET /products/{productId}/batches.
func (h *HTTP) ListProductBatches(ctx context.Context, params oas.ListProductBatchesParams) (*oas.BatchList, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	batches, total, err := h.svc.BatchPage(ctx, point, params.ProductId, params.InStock.Or(false),
		catalog.Page{Limit: params.Limit.Or(50), Offset: params.Offset.Or(0)})
	if err != nil {
		return nil, err
	}
	list := &oas.BatchList{Items: make([]oas.Batch, len(batches)), Total: total}
	numbered := make([]*oas.Batch, len(batches))
	for i, batch := range batches {
		list.Items[i] = *batchResponse(batch)
		numbered[i] = &list.Items[i]
	}
	return list, h.numbered(ctx, numbered...)
}

// GetBatch реализует GET /batches/{batchId}.
func (h *HTTP) GetBatch(ctx context.Context, params oas.GetBatchParams) (*oas.Batch, error) {
	batch, err := h.svc.BatchAt(ctx, params.PointId, params.BatchId)
	if err != nil {
		return nil, err
	}
	return h.batchOut(ctx, batch)
}

// WriteOffStock реализует POST /batches/{batchId}/write-offs.
func (h *HTTP) WriteOffStock(ctx context.Context, req *oas.WriteOff, params oas.WriteOffStockParams) (*oas.Batch, error) {
	actor, err := h.batchWritable(ctx, params.PointId, params.BatchId)
	if err != nil {
		return nil, err
	}
	batch, err := h.svc.WriteOffStock(ctx, actor, params.IdempotencyKey.Or(""), WriteOff{
		BatchID: params.BatchId, Quantity: req.Quantity, Reason: WriteOffReason(req.Reason), Comment: req.Comment,
	})
	if err != nil {
		return nil, err
	}
	return h.batchOut(ctx, batch)
}

// AdjustStock реализует POST /batches/{batchId}/adjustments.
func (h *HTTP) AdjustStock(ctx context.Context, req *oas.Adjustment, params oas.AdjustStockParams) (*oas.Batch, error) {
	actor, err := h.batchWritable(ctx, params.PointId, params.BatchId)
	if err != nil {
		return nil, err
	}
	batch, err := h.svc.AdjustStock(ctx, actor, params.IdempotencyKey.Or(""), Adjustment{
		BatchID: params.BatchId, Delta: req.Delta, Reason: req.Reason, Comment: req.Comment.Or(""),
	})
	if err != nil {
		return nil, err
	}
	return h.batchOut(ctx, batch)
}

// ListDeliveries реализует GET /deliveries.
func (h *HTTP) ListDeliveries(ctx context.Context, params oas.ListDeliveriesParams) (*oas.DeliveryPage, error) {
	return platformredis.LoadProjectionJSON(ctx, h.cache, "inventory-deliveries", "deliveries", params,
		func(ctx context.Context) (*oas.DeliveryPage, error) { return h.listDeliveries(ctx, params) })
}

func (h *HTTP) listDeliveries(ctx context.Context, params oas.ListDeliveriesParams) (*oas.DeliveryPage, error) {
	// значения по умолчанию подставляет сгенерированный разбор запроса
	deliveries, total, err := h.svc.Deliveries(ctx, params.PointId,
		DeliveryFilter{
			Status: DeliveryStatus(params.Status.Or("")), OpenQuarantine: params.Quarantine.Set,
			NumberPrefix: params.Number.Or(""), OldestFirst: params.Sort.Value == oas.ListDeliveriesSortDeliveredAt,
		},
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.DeliveryPage{Items: make([]oas.Delivery, len(deliveries)), Total: total}
	named := make([]*oas.Delivery, len(deliveries))
	for i, delivery := range deliveries {
		page.Items[i] = *deliveryResponse(delivery)
		named[i] = &page.Items[i]
	}
	return page, h.named(ctx, named...)
}

// GetDeliveriesSummary реализует GET /deliveries/summary.
func (h *HTTP) GetDeliveriesSummary(ctx context.Context, params oas.GetDeliveriesSummaryParams) (*oas.DeliverySummary, error) {
	summary, err := h.svc.DeliverySummary(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	return &oas.DeliverySummary{
		Total: summary.Total, Registered: summary.Registered, Approved: summary.Approved,
		OpenQuarantineItems: summary.OpenQuarantineItems,
	}, nil
}

func (h *HTTP) deliveryOut(ctx context.Context, d Delivery) (*oas.Delivery, error) {
	out := deliveryResponse(d)
	return out, h.named(ctx, out)
}

func (h *HTTP) batchOut(ctx context.Context, b Batch) (*oas.Batch, error) {
	out := batchResponse(b)
	return out, h.numbered(ctx, out)
}

// numbered подставляет партиям номер поставки: в складских событиях и таблице партий его нет.
// Новая операция, которая возвращает партию, обязана пройти через неё.
func (h *HTTP) numbered(ctx context.Context, batches ...*oas.Batch) error {
	deliveryIDs := make([]uuid.UUID, len(batches))
	for i, batch := range batches {
		deliveryIDs[i] = batch.DeliveryID
	}
	numbers, err := h.svc.DeliveryNumbers(ctx, deliveryIDs)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		batch.DeliveryNumber = numbers[batch.DeliveryID]
	}
	return nil
}

// named подставляет в поставки то, чего нет в событиях их потока: название, единицу и категорию
// продукта (клиенту иначе пришлось бы читать ради них каталог), текущий остаток партий позиции
// и имена авторов.
func (h *HTTP) named(ctx context.Context, deliveries ...*oas.Delivery) error {
	var (
		productIDs, itemIDs []uuid.UUID
		actors              []*oas.Actor
	)
	for _, delivery := range deliveries {
		if delivery.ApprovedBy.Set {
			actors = append(actors, &delivery.ApprovedBy.Value)
		}
		for i := range delivery.Items {
			item := &delivery.Items[i]
			productIDs, itemIDs = append(productIDs, item.ProductID), append(itemIDs, item.ID)
			for j := range item.QuarantineResolutions {
				actors = append(actors, &item.QuarantineResolutions[j].DecidedBy)
			}
		}
	}
	products, err := h.svc.products.ProductsByID(ctx, productIDs)
	if err != nil {
		return err
	}
	stock, err := h.svc.ItemStock(ctx, itemIDs)
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		for i := range delivery.Items {
			item := &delivery.Items[i]
			item.InStockQuantity = stock[item.ID]
			if product, found := products[item.ProductID]; found {
				item.ProductName = oas.NewOptString(product.Name)
				item.Unit = oas.NewOptUnit(oas.Unit(product.Unit))
				item.ProductCategory = oas.NewOptString(product.Category)
			}
		}
	}
	return h.withNames(ctx, actors)
}

func (h *HTTP) withNames(ctx context.Context, actors []*oas.Actor) error {
	return h.points.WithNames(ctx, actors)
}

func actorResponse(a eventstore.Actor) oas.Actor {
	return oas.Actor{Type: oas.ActorType(a.Type), ID: a.ID}
}

func deliveryResponse(d Delivery) *oas.Delivery {
	out := &oas.Delivery{
		ID: d.ID, PointID: d.PointID, Number: d.Number, DeliveredAt: d.DeliveredAt,
		Status: oas.DeliveryStatus(d.Status), Items: make([]oas.DeliveryItem, len(d.Items)),
	}
	if d.ApprovedAt != nil {
		out.ApprovedAt = oas.NewOptDateTime(*d.ApprovedAt)
	}
	if d.ApprovedBy != nil {
		out.ApprovedBy = oas.NewOptActor(actorResponse(*d.ApprovedBy))
	}
	if d.CorrectedAt != nil {
		out.CorrectedAt = oas.NewOptDateTime(*d.CorrectedAt)
	}
	for i, item := range d.Items {
		outItem := oas.DeliveryItem{
			ID: item.ID, ProductID: item.ProductID, DeclaredQuantity: item.DeclaredQuantity, ExpiresAt: item.ExpiresAt,
			AcceptedQuantity: item.AcceptedQuantity, RejectedQuantity: item.RejectedQuantity,
			QuarantinedQuantity: item.QuarantinedQuantity, Expired: item.Expired,
			// пустой список, а не null: поле обязательное
			BatchIds:              append(make([]uuid.UUID, 0, len(item.BatchIDs)), item.BatchIDs...),
			QuarantineResolutions: make([]oas.QuarantineDecision, len(item.Resolutions)),
		}
		for j, r := range item.Resolutions {
			decision := oas.QuarantineDecision{
				DecidedAt: r.DecidedAt, DecidedBy: actorResponse(r.DecidedBy),
				AcceptedQuantity: r.AcceptedQuantity, RejectedQuantity: r.RejectedQuantity,
			}
			if r.RejectReason != "" {
				decision.RejectReason = oas.NewOptString(r.RejectReason)
			}
			if r.Comment != "" {
				decision.Comment = oas.NewOptString(r.Comment)
			}
			outItem.QuarantineResolutions[j] = decision
		}
		if item.QuarantinedAt != nil {
			outItem.QuarantinedAt = oas.NewOptDateTime(*item.QuarantinedAt)
		}
		if item.Approved != nil {
			outItem.Approved = oas.NewOptItemDistribution(oas.ItemDistribution{
				AcceptedQuantity: item.Approved.AcceptedQuantity, RejectedQuantity: item.Approved.RejectedQuantity,
				QuarantinedQuantity: item.Approved.QuarantinedQuantity,
			})
		}
		if item.ManufacturedAt != nil {
			outItem.ManufacturedAt = oas.NewOptDateTime(*item.ManufacturedAt)
		}
		if item.RejectReason != "" {
			outItem.RejectReason = oas.NewOptString(item.RejectReason)
		}
		if item.QuarantineReason != "" {
			outItem.QuarantineReason = oas.NewOptString(item.QuarantineReason)
		}
		if item.Comment != "" {
			outItem.Comment = oas.NewOptString(item.Comment)
		}
		out.Items[i] = outItem
	}
	return out
}

func batchResponse(b Batch) *oas.Batch {
	out := &oas.Batch{
		ID: b.ID, PointID: b.PointID, ProductID: b.ProductID, DeliveryID: b.DeliveryID, ItemID: b.ItemID,
		ReceivedQuantity: b.ReceivedQuantity, Quantity: b.Quantity,
		ExpiresAt: b.ExpiresAt, ReceivedAt: b.ReceivedAt,
	}
	if b.ExpiryWarnedAt != nil {
		out.ExpiryWarnedAt = oas.NewOptDateTime(*b.ExpiryWarnedAt)
	}
	return out
}
