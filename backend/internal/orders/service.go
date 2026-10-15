package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders/db"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

// errRecipesChanged - рецептуры изменились между чтением и взятием блокировок так,
// что заказу нужны продукты вне заблокированных потоков. Заказ повторяется с новым набором.
var errRecipesChanged = errors.New("orders: recipes changed while the order was being placed")

// maxPlacements ограничивает повторы оформления при изменении рецептур.
const maxPlacements = 3

// Service оформляет заказы.
type Service struct {
	runner *command.Runner
	pool   *pgxpool.Pool
	dishes *catalog.Service
	stock  *inventory.Service
}

func NewService(runner *command.Runner, pool *pgxpool.Pool, dishes *catalog.Service, stock *inventory.Service) *Service {
	return &Service{runner: runner, pool: pool, dishes: dishes, stock: stock}
}

// PlaceOrder оформляет заказ на точке: списывает её продукты по FEFO или отклоняет заказ целиком,
// если хотя бы одного продукта не хватает. Отказ возвращается как заказ со статусом "rejected".
// key - ключ идемпотентности, пустой отключает её.
//
// Блюда - только компании точки и не выключенные на ней: иначе запрос отклоняется без события заказа.
func (s *Service) PlaceOrder(ctx context.Context, actor eventstore.Actor, key string, point organization.Point, items []OrderItem) (Order, error) {
	if err := validateItems(items); err != nil {
		return Order{}, err
	}
	// Повтор уже оформленного заказа возвращает его до проверок блюд: блюдо могли выключить
	// на точке уже после того, как заказ был принят.
	if order, found, err := s.Placed(ctx, actor, key, point, items); err != nil || found {
		return order, err
	}
	orderID := uuid.Must(uuid.NewV7())

	for range maxPlacements {
		// Набор блокировок нужно знать до транзакции, поэтому рецептуры читаются заранее.
		dishes, err := s.loadDishes(ctx, s.pool, point, items)
		if err != nil {
			return Order{}, err
		}
		needs, err := requirements(dishes, items)
		if err != nil {
			return Order{}, err
		}

		order, err := s.place(ctx, actor, key, point, orderID, items, needs)
		if !errors.Is(err, errRecipesChanged) {
			return order, err
		}
	}
	return Order{}, fmt.Errorf("orders: recipes keep changing: %w", eventstore.ErrVersionConflict)
}

// Placed возвращает заказ, уже оформленный запросом с этим ключом идемпотентности.
// Нужен там, где проверка стоит раньше команды: отключённая точка, выключенное блюдо.
func (s *Service) Placed(ctx context.Context, actor eventstore.Actor, key string, point organization.Point, items []OrderItem) (Order, bool, error) {
	return command.Stored[Order](ctx, s.runner, orderSpec(actor, key, point, items, nil))
}

func orderSpec(actor eventstore.Actor, key string, point organization.Point, items []OrderItem, locks []string) command.Spec {
	return command.Spec{
		Name: "PlaceOrder", Actor: actor, Locks: locks, IdempotencyKey: key,
		Request: struct {
			PointID uuid.UUID
			Items   []OrderItem
		}{point.ID, items},
	}
}

// place выполняет одну попытку оформления под блокировками складских потоков из expected.
func (s *Service) place(
	ctx context.Context, actor eventstore.Actor, key string, point organization.Point,
	orderID uuid.UUID, items []OrderItem, expected []inventory.Need,
) (Order, error) {
	locks := make([]string, len(expected))
	for i, need := range expected {
		locks[i] = inventory.StockStream(point.ID, need.ProductID)
	}
	return command.Run(ctx, s.runner, orderSpec(actor, key, point, items, locks), func(ctx context.Context, tx *command.Tx) (Order, error) {
		// Под блокировками рецептуры читаются заново: списывать нужно по актуальным.
		dishes, err := s.loadDishes(ctx, tx, point, items)
		if err != nil {
			return Order{}, err
		}
		needs, err := requirements(dishes, items)
		if err != nil {
			return Order{}, err
		}
		for _, need := range needs {
			if !tx.Locked(inventory.StockStream(point.ID, need.ProductID)) {
				return Order{}, errRecipesChanged
			}
		}

		short, err := s.stock.ConsumeForOrder(ctx, tx, point.ID, orderID, needs)
		if err != nil {
			return Order{}, err
		}

		order := Order{ID: orderID, PointID: point.ID, Status: OrderAcceptedStatus, PlacedAt: tx.Now().UTC(), Items: items, PlacedBy: actor}
		var event eventstore.Payload = OrderAccepted{PointID: point.ID, OrderID: orderID, AcceptedBy: actor, Items: items}
		if len(short) > 0 {
			order.Status = OrderRejectedStatus
			order.Shortages = make([]Shortage, len(short))
			for i, sh := range short {
				order.Shortages[i] = Shortage{
					ProductID: sh.ProductID, RequiredQuantity: sh.Required, AvailableQuantity: sh.Available,
				}
			}
			// отказ фиксируется событием: транзакция завершается успешно, остатки при этом не тронуты
			event = OrderRejected{PointID: point.ID, OrderID: orderID, RejectedBy: actor, Items: items, Shortages: order.Shortages}
		}
		if _, err := tx.Append(ctx, OrderStream(orderID), 0, event); err != nil {
			return Order{}, err
		}
		return order, nil
	})
}

// Order возвращает заказ или ErrNotFound.
func (s *Service) Order(ctx context.Context, id uuid.UUID) (Order, error) {
	order, _, err := loadOrder(ctx, s.pool, id)
	return order, err
}

// loadOrder собирает заказ из его потока и возвращает версию потока.
func loadOrder(ctx context.Context, q eventstore.Querier, id uuid.UUID) (Order, int64, error) {
	stored, err := eventstore.ReadStream(ctx, q, OrderStream(id), 1)
	if err != nil {
		return Order{}, 0, err
	}
	if len(stored) == 0 {
		return Order{}, 0, fmt.Errorf("order %s: %w", id, ErrNotFound)
	}
	order, err := foldOrder(stored)
	return order, int64(len(stored)), err
}

// CancelOrder отменяет принятый заказ точки и возвращает в партии всё, что взяли под него.
// key - ключ идемпотентности, пустой отключает её; comment - необязательный комментарий.
func (s *Service) CancelOrder(ctx context.Context, actor eventstore.Actor, key string, point organization.Point, orderID uuid.UUID, comment string) (Order, error) {
	comment, err := cancelComment(comment)
	if err != nil {
		return Order{}, err
	}
	// расход заказа неизменен: по нему заранее берутся блокировки складских потоков
	consumed, err := s.stock.OrderConsumption(ctx, point.ID, orderID)
	if err != nil {
		return Order{}, err
	}
	locks := []string{OrderStream(orderID)}
	for _, taken := range consumed {
		locks = append(locks, inventory.StockStream(point.ID, taken.ProductID))
	}
	spec := command.Spec{
		Name: "CancelOrder", Actor: actor, Locks: locks, IdempotencyKey: key,
		// без комментария запрос совпадает с прежним: ключи, выданные до его появления, повторяются
		Request: struct {
			PointID, OrderID uuid.UUID
			Comment          string `json:",omitempty"`
		}{point.ID, orderID, comment},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Order, error) {
		order, version, err := loadOrder(ctx, tx, orderID)
		if err != nil {
			return Order{}, err
		}
		if order.PointID != point.ID {
			return Order{}, fmt.Errorf("order %s: %w", orderID, ErrNotFound)
		}
		// учётная запись сервиса (Мигома) одна на всех его клиентов: чужие заказы точки ей не отменить
		if actor.Type == eventstore.ActorService && order.PlacedBy != actor {
			return Order{}, fmt.Errorf("order %s placed by another client: %w", orderID, organization.ErrForbidden)
		}
		if order.Status != OrderAcceptedStatus {
			return Order{}, ErrNotCancellable
		}
		cancelled := OrderCancelled{PointID: point.ID, OrderID: orderID, CancelledBy: actor, Comment: comment}
		if _, err := tx.Append(ctx, OrderStream(orderID), version, cancelled); err != nil {
			return Order{}, err
		}
		if err := s.stock.ReturnForOrder(ctx, tx, actor, consumed); err != nil {
			return Order{}, err
		}
		cancelledAt := tx.Now().UTC()
		order.Status, order.CancelledAt = OrderCancelledStatus, &cancelledAt
		order.CancelledBy, order.CancelComment = &actor, comment
		return order, nil
	})
}

// OrderAt возвращает заказ точки. Заказ другой точки - ErrNotFound.
func (s *Service) OrderAt(ctx context.Context, pointID, id uuid.UUID) (Order, error) {
	order, err := s.Order(ctx, id)
	if err == nil && order.PointID != pointID {
		return Order{}, fmt.Errorf("order %s: %w", id, ErrNotFound)
	}
	return order, err
}

// OrderFilter - условия списка заказов. Пустое значение не фильтрует.
type OrderFilter struct {
	Status OrderStatus
}

// Orders возвращает страницу истории заказов точки от поздних к ранним и общее число заказов по фильтру.
// История строится асинхронной проекцией и может отставать от записи.
func (s *Service) Orders(ctx context.Context, pointID uuid.UUID, filter OrderFilter, page catalog.Page) ([]Order, int64, error) {
	var status *string
	if filter.Status != "" {
		value := string(filter.Status)
		status = &value
	}
	q := db.New(s.pool)
	rows, err := q.ListOrders(ctx, db.ListOrdersParams{PointID: pointID, Status: status, PageLimit: page.Limit, PageOffset: page.Offset})
	if err != nil {
		return nil, 0, fmt.Errorf("orders: list orders: %w", err)
	}
	total, err := q.CountOrders(ctx, db.CountOrdersParams{PointID: pointID, Status: status})
	if err != nil {
		return nil, 0, fmt.Errorf("orders: count orders: %w", err)
	}
	orders := make([]Order, len(rows))
	for i, state := range rows {
		if err := json.Unmarshal(state, &orders[i]); err != nil {
			return nil, 0, fmt.Errorf("orders: decode order: %w", err)
		}
	}
	return orders, total, nil
}

// loadDishes читает блюда заказа. Блюдо другой компании для точки не существует,
// блюдо, выключенное на точке, на ней не продаётся.
func (s *Service) loadDishes(
	ctx context.Context, q catalog.Querier, point organization.Point, items []OrderItem,
) (map[uuid.UUID]catalog.Dish, error) {
	dishes := make(map[uuid.UUID]catalog.Dish, len(items))
	ids := make([]uuid.UUID, len(items))
	for i, item := range items {
		ids[i] = item.DishID
		dish, err := s.dishes.DishIn(ctx, q, item.DishID)
		if errors.Is(err, catalog.ErrNotFound) || (err == nil && dish.CompanyID != point.CompanyID) {
			return nil, &UnknownDishError{DishID: item.DishID}
		}
		if err != nil {
			return nil, err
		}
		dishes[item.DishID] = dish
	}
	disabled, err := s.dishes.DisabledOnPoint(ctx, q, point.ID, ids)
	if err != nil {
		return nil, err
	}
	if len(disabled) > 0 {
		return nil, &DishNotSoldError{DishID: disabled[0]}
	}
	return dishes, nil
}
