//go:build integration

package orders_test

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
)

var (
	startOfDay = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	oct        = func(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }
)

type fixture struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	clock   *clock.Fake
	catalog *catalog.Service
	stock   *inventory.Service
	svc     *orders.Service
}

func setup(t *testing.T) fixture {
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(startOfDay)
	runner := command.NewRunner(pool, clk, slog.New(slog.DiscardHandler))
	cat := catalog.NewService(runner, pool)
	stock := inventory.NewService(runner, pool, cat)
	return fixture{context.Background(), pool, clk, cat, stock, orders.NewService(runner, pool, cat, stock)}
}

func (f fixture) product(t *testing.T, name string) catalog.Product {
	t.Helper()
	p, err := f.catalog.CreateProduct(f.ctx, operator, "", companyID, catalog.NewProduct{Name: name, Unit: catalog.UnitGram})
	require.NoError(t, err)
	return p
}

// dish создаёт блюдо с рецептурой: пары "продукт, количество на порцию".
func (f fixture) dish(t *testing.T, name string, ingredients ...catalog.Ingredient) catalog.Dish {
	t.Helper()
	d, err := f.catalog.CreateDish(f.ctx, operator, "", companyID, name)
	require.NoError(t, err)
	if len(ingredients) == 0 {
		return d
	}
	d, err = f.catalog.ChangeRecipe(f.ctx, operator, "", d.ID, ingredients)
	require.NoError(t, err)
	return d
}

// supply принимает поставку одного продукта и возвращает созданную партию.
func (f fixture) supply(t *testing.T, product catalog.Product, quantity int64, expiresAt time.Time) uuid.UUID {
	t.Helper()
	d, err := f.stock.RegisterDelivery(f.ctx, operator, "", point, inventory.NewDelivery{
		Number: "ПН-1", DeliveredAt: f.clock.Now(),
		Items: []inventory.NewDeliveryItem{{ProductID: product.ID, DeclaredQuantity: quantity, ExpiresAt: expiresAt}},
	})
	require.NoError(t, err)
	d, err = f.stock.ApproveDelivery(f.ctx, operator, "", d.ID, []inventory.ItemDecision{
		{ItemID: d.Items[0].ID, AcceptedQuantity: quantity},
	})
	require.NoError(t, err)
	return d.Items[0].BatchIDs[0]
}

func (f fixture) place(t *testing.T, key string, items ...orders.OrderItem) orders.Order {
	t.Helper()
	order, err := f.svc.PlaceOrder(f.ctx, operator, key, point, items)
	require.NoError(t, err)
	return order
}

// remaining возвращает остатки партий продукта в порядке FEFO.
func (f fixture) remaining(t *testing.T, product catalog.Product) []int64 {
	t.Helper()
	batches, err := f.stock.Batches(f.ctx, point.ID, product.ID)
	require.NoError(t, err)
	quantities := make([]int64, len(batches))
	for i, b := range batches {
		quantities[i] = b.Quantity
	}
	return quantities
}

func (f fixture) events(t *testing.T, stream string) []eventstore.Payload {
	t.Helper()
	stored, err := eventstore.ReadStream(f.ctx, f.pool, stream, 1)
	require.NoError(t, err)
	payloads := make([]eventstore.Payload, len(stored))
	for i, e := range stored {
		payloads[i], err = eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
		require.NoError(t, err)
	}
	return payloads
}

func (f fixture) totalEvents(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM events").Scan(&n))
	return n
}

func per(product catalog.Product, quantity int64) catalog.Ingredient {
	return catalog.Ingredient{ProductID: product.ID, Quantity: quantity}
}

func portions(dish catalog.Dish, n int64) orders.OrderItem {
	return orders.OrderItem{DishID: dish.ID, Portions: n}
}

// Состав заказа.

func TestPlaceOrderRejectsInvalidComposition(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 80))
	draft := f.dish(t, "Черновик")
	f.supply(t, milk, 1000, oct(20))
	before := f.totalEvents(t)

	t.Run("empty", func(t *testing.T) {
		_, err := f.svc.PlaceOrder(f.ctx, operator, "", point, nil)
		var validation *orders.ValidationError
		require.ErrorAs(t, err, &validation)
	})
	t.Run("zero portions", func(t *testing.T) {
		_, err := f.svc.PlaceOrder(f.ctx, operator, "", point, []orders.OrderItem{portions(pasta, 0)})
		var validation *orders.ValidationError
		require.ErrorAs(t, err, &validation)
	})
	t.Run("dish twice", func(t *testing.T) {
		_, err := f.svc.PlaceOrder(f.ctx, operator, "", point, []orders.OrderItem{portions(pasta, 1), portions(pasta, 1)})
		var validation *orders.ValidationError
		require.ErrorAs(t, err, &validation)
	})
	t.Run("unknown dish", func(t *testing.T) {
		unknown := uuid.Must(uuid.NewV7())
		_, err := f.svc.PlaceOrder(f.ctx, operator, "", point, []orders.OrderItem{{DishID: unknown, Portions: 1}})
		var unknownDish *orders.UnknownDishError
		require.ErrorAs(t, err, &unknownDish)
		require.Equal(t, unknown, unknownDish.DishID)
	})
	t.Run("dish without recipe", func(t *testing.T) {
		_, err := f.svc.PlaceOrder(f.ctx, operator, "", point, []orders.OrderItem{portions(pasta, 1), portions(draft, 1)})
		var noRecipe *orders.DishWithoutRecipeError
		require.ErrorAs(t, err, &noRecipe)
		require.Equal(t, draft.ID, noRecipe.DishID)
	})

	require.Equal(t, before, f.totalEvents(t))
	require.Equal(t, []int64{1000}, f.remaining(t, milk))
}

// Принятие.

func TestPlaceOrderAccepted(t *testing.T) {
	f := setup(t)
	milk, ham := f.product(t, "Сливки"), f.product(t, "Бекон")
	pasta := f.dish(t, "Паста", per(milk, 80), per(ham, 60))
	soup := f.dish(t, "Суп", per(milk, 50))
	milkBatch := f.supply(t, milk, 1000, oct(20))
	hamBatch := f.supply(t, ham, 500, oct(20))

	order := f.place(t, "", portions(pasta, 2), portions(soup, 1))

	require.Equal(t, orders.OrderAcceptedStatus, order.Status)
	require.Empty(t, order.Shortages)
	require.Equal(t, []orders.OrderItem{portions(pasta, 2), portions(soup, 1)}, order.Items)
	// сливки входят в оба блюда: 2*80 + 1*50
	require.Equal(t, []int64{1000 - 210}, f.remaining(t, milk))
	require.Equal(t, []int64{500 - 120}, f.remaining(t, ham))

	read, err := f.svc.Order(f.ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, order, read)

	require.Equal(t, []eventstore.Payload{orders.OrderAccepted{
		PointID: point.ID, OrderID: order.ID, AcceptedBy: operator, Items: order.Items,
	}}, f.events(t, orders.OrderStream(order.ID)))

	milkEvents := f.events(t, inventory.StockStream(point.ID, milk.ID))
	require.Equal(t, inventory.StockConsumed{
		PointID: point.ID, ProductID: milk.ID, OrderID: order.ID,
		Batches: []inventory.BatchQuantity{{BatchID: milkBatch, Quantity: 210}},
	}, milkEvents[len(milkEvents)-1])
	hamEvents := f.events(t, inventory.StockStream(point.ID, ham.ID))
	require.Equal(t, inventory.StockConsumed{
		PointID: point.ID, ProductID: ham.ID, OrderID: order.ID,
		Batches: []inventory.BatchQuantity{{BatchID: hamBatch, Quantity: 120}},
	}, hamEvents[len(hamEvents)-1])
}

func TestPlaceOrderForWholeStock(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 80))
	f.supply(t, milk, 160, oct(20))

	order := f.place(t, "", portions(pasta, 2))

	require.Equal(t, orders.OrderAcceptedStatus, order.Status)
	require.Equal(t, []int64{0}, f.remaining(t, milk))
}

// Отказ.

func TestPlaceOrderRejectedOnShortage(t *testing.T) {
	f := setup(t)
	milk, ham, egg := f.product(t, "Сливки"), f.product(t, "Бекон"), f.product(t, "Яйцо")
	pasta := f.dish(t, "Паста", per(milk, 80), per(ham, 60), per(egg, 1))
	f.supply(t, milk, 1000, oct(20))
	f.supply(t, ham, 50, oct(20))
	// яиц на складе нет вовсе
	milkEventsBefore := len(f.events(t, inventory.StockStream(point.ID, milk.ID)))

	order := f.place(t, "", portions(pasta, 2))

	require.Equal(t, orders.OrderRejectedStatus, order.Status)
	require.ElementsMatch(t, []orders.Shortage{
		{ProductID: ham.ID, RequiredQuantity: 120, AvailableQuantity: 50},
		{ProductID: egg.ID, RequiredQuantity: 2, AvailableQuantity: 0},
	}, order.Shortages, "every missing product is listed, not only the first one")

	// остатки не тронуты, включая продукт, которого хватало
	require.Equal(t, []int64{1000}, f.remaining(t, milk))
	require.Equal(t, []int64{50}, f.remaining(t, ham))
	require.Len(t, f.events(t, inventory.StockStream(point.ID, milk.ID)), milkEventsBefore, "no StockConsumed")

	events := f.events(t, orders.OrderStream(order.ID))
	require.Len(t, events, 1)
	rejected := events[0].(orders.OrderRejected)
	require.Equal(t, operator, rejected.RejectedBy)
	require.Equal(t, order.Items, rejected.Items)
	require.Equal(t, order.Shortages, rejected.Shortages)

	read, err := f.svc.Order(f.ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, order, read)
}

func TestOrderNotFound(t *testing.T) {
	f := setup(t)

	_, err := f.svc.Order(f.ctx, uuid.Must(uuid.NewV7()))

	require.ErrorIs(t, err, orders.ErrNotFound)
}

// FEFO и сроки годности.

func TestPlaceOrderConsumesByExpiry(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 50))
	late := f.supply(t, milk, 100, oct(20))
	early := f.supply(t, milk, 100, oct(12))
	middle := f.supply(t, milk, 100, oct(15))

	order := f.place(t, "", portions(pasta, 5)) // 250

	require.Equal(t, orders.OrderAcceptedStatus, order.Status)
	require.Equal(t, []int64{0, 0, 50}, f.remaining(t, milk), "batches in FEFO order: Oct 12, 15, 20")
	events := f.events(t, inventory.StockStream(point.ID, milk.ID))
	require.Equal(t, []inventory.BatchQuantity{
		{BatchID: early, Quantity: 100}, {BatchID: middle, Quantity: 100}, {BatchID: late, Quantity: 50},
	}, events[len(events)-1].(inventory.StockConsumed).Batches)
}

func TestPlaceOrderSkipsExpiredBatch(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 50))
	f.supply(t, milk, 100, oct(12))
	f.supply(t, milk, 100, oct(15))
	f.supply(t, milk, 100, oct(20))

	f.clock.Set(oct(12)) // срок первой партии истекает в этот момент: она уже просрочена

	order := f.place(t, "", portions(pasta, 3)) // 150
	require.Equal(t, orders.OrderAcceptedStatus, order.Status)
	require.Equal(t, []int64{100, 0, 50}, f.remaining(t, milk), "expired batch stays untouched")

	// годных осталось 50, просроченные 100 в доступный остаток не входят
	rejected := f.place(t, "", portions(pasta, 2)) // 100
	require.Equal(t, orders.OrderRejectedStatus, rejected.Status)
	require.Equal(t, []orders.Shortage{{ProductID: milk.ID, RequiredQuantity: 100, AvailableQuantity: 50}}, rejected.Shortages)
	require.Equal(t, []int64{100, 0, 50}, f.remaining(t, milk))
}

// Параллельность.

func TestConcurrentOrdersCompeteForStock(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 80))
	f.supply(t, milk, 300, oct(12))
	f.supply(t, milk, 200, oct(15))

	var accepted, rejected atomic.Int32
	var g errgroup.Group
	for range 10 {
		g.Go(func() error {
			order, err := f.svc.PlaceOrder(f.ctx, operator, "", point, []orders.OrderItem{portions(pasta, 1)})
			if err != nil {
				return err
			}
			if order.Status == orders.OrderAcceptedStatus {
				accepted.Add(1)
			} else {
				rejected.Add(1)
			}
			return nil
		})
	}
	require.NoError(t, g.Wait())

	require.Equal(t, int32(6), accepted.Load(), "500 / 80")
	require.Equal(t, int32(4), rejected.Load())
	var total int64
	for _, quantity := range f.remaining(t, milk) {
		require.GreaterOrEqual(t, quantity, int64(0))
		total += quantity
	}
	require.Equal(t, int64(20), total)

	// читатели получают события складского потока в порядке версий
	all, err := eventstore.ReadAll(f.ctx, f.pool, eventstore.Checkpoint{}, 1000)
	require.NoError(t, err)
	var lastVersion int64
	for _, e := range all {
		if e.StreamID == inventory.StockStream(point.ID, milk.ID) {
			require.Equal(t, lastVersion+1, e.Version)
			lastVersion = e.Version
		}
	}
	require.Equal(t, int64(2+6), lastVersion, "two batches and six consumptions")
}

func TestConcurrentOrdersForDifferentProducts(t *testing.T) {
	f := setup(t)
	var dishes []catalog.Dish
	var products []catalog.Product
	for _, name := range []string{"Сливки", "Бекон", "Яйцо", "Сыр"} {
		p := f.product(t, name)
		f.supply(t, p, 100, oct(20))
		products = append(products, p)
		dishes = append(dishes, f.dish(t, "Блюдо из: "+name, per(p, 100)))
	}

	var g errgroup.Group
	for _, dish := range dishes {
		g.Go(func() error {
			order, err := f.svc.PlaceOrder(f.ctx, operator, "", point, []orders.OrderItem{portions(dish, 1)})
			if err == nil && order.Status != orders.OrderAcceptedStatus {
				t.Errorf("order for %s was rejected", dish.Name)
			}
			return err
		})
	}

	require.NoError(t, g.Wait())
	for _, p := range products {
		require.Equal(t, []int64{0}, f.remaining(t, p))
	}
}

// Идемпотентность.

func TestPlaceOrderIsIdempotent(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 80))
	f.supply(t, milk, 1000, oct(20))

	first := f.place(t, "key-1", portions(pasta, 1))
	second := f.place(t, "key-1", portions(pasta, 1))

	require.Equal(t, first, second)
	require.Equal(t, []int64{920}, f.remaining(t, milk), "consumed once")
}

func TestRejectedOrderKeepsItsIdempotencyKey(t *testing.T) {
	f := setup(t)
	milk := f.product(t, "Сливки")
	pasta := f.dish(t, "Паста", per(milk, 80))

	rejected := f.place(t, "key-1", portions(pasta, 1))
	require.Equal(t, orders.OrderRejectedStatus, rejected.Status)

	f.supply(t, milk, 1000, oct(20))
	again := f.place(t, "key-1", portions(pasta, 1))
	require.Equal(t, rejected, again, "the same key returns the stored rejection")
	require.Equal(t, []int64{1000}, f.remaining(t, milk))

	// новый заказ оформляется с новым ключом
	fresh := f.place(t, "key-2", portions(pasta, 1))
	require.Equal(t, orders.OrderAcceptedStatus, fresh.Status)
	require.Equal(t, []int64{920}, f.remaining(t, milk))
}
