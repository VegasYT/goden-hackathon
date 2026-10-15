package orders

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

// HTTP реализует операции заказов из api/openapi.yaml. Обработчик - тонкая обёртка:
// разбирает запрос, вызывает сервис, собирает ответ. Правил в нём нет.
//
// Операции лежат под /points/{pointId}: право на точке проверил слой API, обработчик
// сверяет заказ из адреса с точкой и оформляет заказы только на работающей точке.
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

// PlaceOrder реализует POST /orders. Отказ из-за нехватки продуктов - не ошибка:
// заказ возвращается со статусом "rejected" и списком нехваток.
func (h *HTTP) PlaceOrder(ctx context.Context, req *oas.NewOrder, params oas.PlaceOrderParams) (*oas.Order, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("orders: request has no actor")
	}
	items := make([]OrderItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = OrderItem{DishID: item.DishID, Portions: item.Portions}
	}
	key := params.IdempotencyKey.Or("")
	point, err := h.points.WorkingPoint(ctx, params.PointId)
	if errors.Is(err, organization.ErrDisabled) {
		// точку отключили после того, как заказ был принят: повтор запроса возвращает его
		if closed, findErr := h.points.Point(ctx, params.PointId); findErr == nil {
			if order, found, _ := h.svc.Placed(ctx, actor, key, closed, items); found {
				return h.orderOut(ctx, order)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	order, err := h.svc.PlaceOrder(ctx, actor, key, point, items)
	if err != nil {
		return nil, err
	}
	return h.orderOut(ctx, order)
}

// CancelOrder реализует POST /orders/{orderId}/cancel. Тело с комментарием необязательное.
func (h *HTTP) CancelOrder(ctx context.Context, req oas.OptOrderCancellation, params oas.CancelOrderParams) (*oas.Order, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("orders: request has no actor")
	}
	// заказ другой точки - 404 раньше проверки, работает ли точка
	if _, err := h.svc.OrderAt(ctx, params.PointId, params.OrderId); err != nil {
		return nil, err
	}
	point, err := h.points.WorkingPoint(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	order, err := h.svc.CancelOrder(ctx, actor, params.IdempotencyKey.Or(""), point, params.OrderId, req.Value.Comment.Or(""))
	if err != nil {
		return nil, err
	}
	return h.orderOut(ctx, order)
}

// GetOrder реализует GET /orders/{orderId}.
func (h *HTTP) GetOrder(ctx context.Context, params oas.GetOrderParams) (*oas.Order, error) {
	order, err := h.svc.OrderAt(ctx, params.PointId, params.OrderId)
	if err != nil {
		return nil, err
	}
	return h.orderOut(ctx, order)
}

// ListOrders реализует GET /orders.
func (h *HTTP) ListOrders(ctx context.Context, params oas.ListOrdersParams) (*oas.OrderPage, error) {
	return platformredis.LoadProjectionJSON(ctx, h.cache, "orders-history", "orders", params,
		func(ctx context.Context) (*oas.OrderPage, error) { return h.listOrders(ctx, params) })
}

func (h *HTTP) listOrders(ctx context.Context, params oas.ListOrdersParams) (*oas.OrderPage, error) {
	// значения по умолчанию подставляет сгенерированный разбор запроса
	orders, total, err := h.svc.Orders(ctx, params.PointId,
		OrderFilter{Status: OrderStatus(params.Status.Or(""))},
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.OrderPage{Items: make([]oas.Order, len(orders)), Total: total}
	named := make([]*oas.Order, len(orders))
	for i, order := range orders {
		page.Items[i] = *orderResponse(order)
		named[i] = &page.Items[i]
	}
	return page, h.named(ctx, named...)
}

func (h *HTTP) orderOut(ctx context.Context, o Order) (*oas.Order, error) {
	out := orderResponse(o)
	return out, h.named(ctx, out)
}

// named подставляет в позиции заказов название блюда, в отказы - продукт, авторам - имена,
// чтобы клиенту не читать ради них каталог и сотрудников.
func (h *HTTP) named(ctx context.Context, orders ...*oas.Order) error {
	if err := h.withProducts(ctx, orders); err != nil {
		return err
	}
	var actors []*oas.Actor
	for _, order := range orders {
		actors = append(actors, &order.PlacedBy)
		if order.CancelledBy.Set {
			actors = append(actors, &order.CancelledBy.Value)
		}
	}
	if err := h.points.WithNames(ctx, actors); err != nil {
		return err
	}
	names := map[uuid.UUID]string{}
	for _, order := range orders {
		for i := range order.Items {
			id := order.Items[i].DishID
			name, known := names[id]
			if !known {
				dish, err := h.svc.dishes.Dish(ctx, id)
				if err != nil && !errors.Is(err, catalog.ErrNotFound) {
					return err
				}
				name = dish.Name
				names[id] = name
			}
			if name != "" {
				order.Items[i].DishName = oas.NewOptString(name)
			}
		}
	}
	return nil
}

// withProducts подставляет продукт в недостающие продукты отказов одним запросом к каталогу.
func (h *HTTP) withProducts(ctx context.Context, orders []*oas.Order) error {
	var ids []uuid.UUID
	for _, order := range orders {
		for _, shortage := range order.Shortages {
			ids = append(ids, shortage.ProductID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	products, err := h.svc.dishes.ProductsByID(ctx, ids)
	if err != nil {
		return err
	}
	for _, order := range orders {
		for i := range order.Shortages {
			if product, ok := products[order.Shortages[i].ProductID]; ok {
				order.Shortages[i].Product = oas.NewOptProduct(*catalog.ProductResponse(product))
			}
		}
	}
	return nil
}

func orderResponse(o Order) *oas.Order {
	out := &oas.Order{
		ID: o.ID, PointID: o.PointID, Status: oas.OrderStatus(o.Status), PlacedBy: actorResponse(o.PlacedBy),
		Items: make([]oas.OrderItem, len(o.Items)),
		// пустой список, а не null: поле обязательное
		Shortages: make([]oas.Shortage, len(o.Shortages)),
	}
	// у результата команды, сохранённого до появления поля, времени нет
	if !o.PlacedAt.IsZero() {
		out.PlacedAt = oas.NewOptDateTime(o.PlacedAt)
	}
	if o.CancelledAt != nil {
		out.CancelledAt = oas.NewOptDateTime(*o.CancelledAt)
	}
	if o.CancelledBy != nil {
		out.CancelledBy = oas.NewOptActor(actorResponse(*o.CancelledBy))
	}
	if o.CancelComment != "" {
		out.CancelComment = oas.NewOptString(o.CancelComment)
	}
	for i, item := range o.Items {
		out.Items[i] = oas.OrderItem{DishID: item.DishID, Portions: item.Portions}
	}
	for i, shortage := range o.Shortages {
		out.Shortages[i] = oas.Shortage{
			ProductID: shortage.ProductID, RequiredQuantity: shortage.RequiredQuantity, AvailableQuantity: shortage.AvailableQuantity,
		}
	}
	return out
}

func actorResponse(a eventstore.Actor) oas.Actor {
	return oas.Actor{Type: oas.ActorType(a.Type), ID: a.ID}
}
