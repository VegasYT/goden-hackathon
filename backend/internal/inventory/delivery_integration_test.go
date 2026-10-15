//go:build integration

package inventory_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
)

// companyID - компания, в которой тесты пакета создают каталог.
var companyID = point.CompanyID

var (
	startOfDay = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	oct        = func(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }
)

type fixture struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	clock    *clock.Fake
	runner   *command.Runner
	products *catalog.Service
	svc      *inventory.Service
}

func setup(t *testing.T) fixture {
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(startOfDay)
	runner := command.NewRunner(pool, clk, slog.New(slog.DiscardHandler))
	products := catalog.NewService(runner, pool)
	return fixture{context.Background(), pool, clk, runner, products, inventory.NewService(runner, pool, products)}
}

func (f fixture) product(t *testing.T, name string, shelfLifeDays *int) catalog.Product {
	t.Helper()
	p, err := f.products.CreateProduct(f.ctx, manager, "", companyID, catalog.NewProduct{
		Name: name, Unit: catalog.UnitGram, ShelfLifeDays: shelfLifeDays,
	})
	require.NoError(t, err)
	return p
}

func (f fixture) register(t *testing.T, items ...inventory.NewDeliveryItem) inventory.Delivery {
	t.Helper()
	d, err := f.svc.RegisterDelivery(f.ctx, manager, "", point, inventory.NewDelivery{
		Number: "ПН-000123", DeliveredAt: startOfDay.Add(-30 * time.Minute), Items: items,
	})
	require.NoError(t, err)
	return d
}

func item(product catalog.Product, quantity int64, expiresAt time.Time) inventory.NewDeliveryItem {
	return inventory.NewDeliveryItem{ProductID: product.ID, DeclaredQuantity: quantity, ExpiresAt: expiresAt}
}

func acceptAll(d inventory.Delivery) []inventory.ItemDecision {
	decisions := make([]inventory.ItemDecision, len(d.Items))
	for i, it := range d.Items {
		decisions[i] = inventory.ItemDecision{ItemID: it.ID, AcceptedQuantity: it.DeclaredQuantity}
	}
	return decisions
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

func (f fixture) batches(t *testing.T, productID uuid.UUID) []inventory.Batch {
	t.Helper()
	batches, err := f.svc.Batches(f.ctx, point.ID, productID)
	require.NoError(t, err)
	return batches
}

func (f fixture) totalEvents(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM events").Scan(&n))
	return n
}

func requireInvalid(t *testing.T, err error) {
	t.Helper()
	var validation *inventory.ValidationError
	require.ErrorAs(t, err, &validation)
}

// Регистрация.

func TestRegisterDelivery(t *testing.T) {
	f := setup(t)
	cream, bacon := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil)
	made := oct(9)

	registered, err := f.svc.RegisterDelivery(f.ctx, manager, "", point, inventory.NewDelivery{
		Number: " ПН-000123 ", DeliveredAt: startOfDay,
		Items: []inventory.NewDeliveryItem{
			{ProductID: cream.ID, DeclaredQuantity: 1000, ExpiresAt: oct(15), ManufacturedAt: &made},
			item(bacon, 500, oct(20)),
		},
	})
	require.NoError(t, err)

	require.Equal(t, "ПН-000123", registered.Number)
	require.Equal(t, inventory.DeliveryRegisteredStatus, registered.Status)
	require.Nil(t, registered.ApprovedAt)
	require.Len(t, registered.Items, 2)
	require.NotEqual(t, registered.Items[0].ID, registered.Items[1].ID)
	require.Equal(t, &made, registered.Items[0].ManufacturedAt)

	read, err := f.svc.Delivery(f.ctx, registered.ID)
	require.NoError(t, err)
	require.Equal(t, registered, read)

	events := f.events(t, inventory.DeliveryStream(registered.ID))
	require.Len(t, events, 1)
	event := events[0].(inventory.DeliveryRegistered)
	require.Equal(t, manager, event.RegisteredBy)
	require.Len(t, event.Items, 2)
	require.Equal(t, int64(1000), event.Items[0].DeclaredQuantity)

	// регистрация не меняет остаток
	require.Empty(t, f.batches(t, cream.ID))
	require.Empty(t, f.events(t, inventory.StockStream(point.ID, cream.ID)))
}

func TestRegisterDeliveryRejected(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", ptr(5))
	made := oct(1)
	before := f.totalEvents(t)

	t.Run("unknown product", func(t *testing.T) {
		unknown := uuid.Must(uuid.NewV7())
		_, err := f.svc.RegisterDelivery(f.ctx, manager, "", point, inventory.NewDelivery{
			Number: "ПН-1", DeliveredAt: startOfDay,
			Items: []inventory.NewDeliveryItem{{ProductID: unknown, DeclaredQuantity: 1, ExpiresAt: oct(15)}},
		})

		var unknownProduct *catalog.UnknownProductError
		require.ErrorAs(t, err, &unknownProduct)
		require.Equal(t, unknown, unknownProduct.ProductID)
	})

	invalid := map[string]inventory.NewDelivery{
		"no expires_at": {Number: "ПН-1", DeliveredAt: startOfDay, Items: []inventory.NewDeliveryItem{{ProductID: cream.ID, DeclaredQuantity: 1}}},
		"empty number":  {DeliveredAt: startOfDay, Items: []inventory.NewDeliveryItem{item(cream, 1, oct(15))}},
		"no items":      {Number: "ПН-1", DeliveredAt: startOfDay},
		"zero quantity": {Number: "ПН-1", DeliveredAt: startOfDay, Items: []inventory.NewDeliveryItem{item(cream, 0, oct(15))}},
		"beyond shelf life": {Number: "ПН-1", DeliveredAt: startOfDay, Items: []inventory.NewDeliveryItem{
			{ProductID: cream.ID, DeclaredQuantity: 1, ExpiresAt: oct(11), ManufacturedAt: &made},
		}},
	}
	for name, in := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.RegisterDelivery(f.ctx, manager, "", point, in)
			requireInvalid(t, err)
		})
	}

	require.Equal(t, before, f.totalEvents(t))
}

func TestRegisterDeliveryWithExpiredItem(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)

	registered := f.register(t, item(cream, 100, oct(1)))

	require.Equal(t, inventory.DeliveryRegisteredStatus, registered.Status)
}

func TestDeliveryNotFound(t *testing.T) {
	f := setup(t)
	missing := uuid.Must(uuid.NewV7())

	_, err := f.svc.Delivery(f.ctx, missing)
	require.ErrorIs(t, err, inventory.ErrNotFound)

	_, err = f.svc.ApproveDelivery(f.ctx, manager, "", missing, nil)
	require.ErrorIs(t, err, inventory.ErrNotFound)
}

// Приёмка и партии.

func TestApproveDeliveryPartially(t *testing.T) {
	f := setup(t)
	cream, bacon := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil)
	registered := f.register(t, item(cream, 10, oct(15)), item(bacon, 500, oct(20)))

	approved, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, []inventory.ItemDecision{
		{
			ItemID: registered.Items[0].ID, AcceptedQuantity: 6, RejectedQuantity: 3, QuarantinedQuantity: 1,
			RejectReason: "damaged_package", QuarantineReason: "temperature_check", Comment: "три вскрыты, одна тёплая",
		},
		{ItemID: registered.Items[1].ID, RejectedQuantity: 500, RejectReason: "wrong_product", Comment: "не тот сорт"},
	})
	require.NoError(t, err)

	// состояние поставки
	require.Equal(t, inventory.DeliveryApprovedStatus, approved.Status)
	require.Equal(t, startOfDay, *approved.ApprovedAt)
	require.Equal(t, manager, *approved.ApprovedBy)
	first, second := approved.Items[0], approved.Items[1]
	require.Equal(t, []int64{10, 6, 3, 1},
		[]int64{first.DeclaredQuantity, first.AcceptedQuantity, first.RejectedQuantity, first.QuarantinedQuantity})
	require.Equal(t, "damaged_package", first.RejectReason)
	require.Equal(t, "temperature_check", first.QuarantineReason)
	require.Equal(t, "три вскрыты, одна тёплая", first.Comment)
	require.Len(t, first.BatchIDs, 1)
	require.Empty(t, second.BatchIDs, "fully rejected item has no batch")
	read, err := f.svc.Delivery(f.ctx, registered.ID)
	require.NoError(t, err)
	require.Equal(t, approved, read)

	// партия только на принятое количество
	batches := f.batches(t, cream.ID)
	require.Equal(t, []inventory.Batch{{
		PointID: point.ID, ID: first.BatchIDs[0], ProductID: cream.ID, DeliveryID: registered.ID, ItemID: first.ID,
		ReceivedQuantity: 6, Quantity: 6, ExpiresAt: oct(15), ReceivedAt: startOfDay,
	}}, batches)
	require.Empty(t, f.batches(t, bacon.ID))

	// события
	require.Equal(t, []eventstore.Payload{inventory.BatchReceived{
		PointID: point.ID, BatchID: first.BatchIDs[0], ProductID: cream.ID, DeliveryID: registered.ID, ItemID: first.ID,
		Quantity: 6, ExpiresAt: eventstore.At(oct(15)), ReceivedAt: eventstore.At(startOfDay),
		Source: inventory.BatchFromApproval,
	}}, f.events(t, inventory.StockStream(point.ID, cream.ID)))
	require.Empty(t, f.events(t, inventory.StockStream(point.ID, bacon.ID)))

	deliveryEvents := f.events(t, inventory.DeliveryStream(registered.ID))
	require.Len(t, deliveryEvents, 2)
	snapshot := deliveryEvents[1].(inventory.DeliveryApproved)
	require.Equal(t, manager, snapshot.ApprovedBy)
	require.Len(t, snapshot.Items, 2)
	require.Equal(t, int64(3), snapshot.Items[0].RejectedQuantity)
	require.Equal(t, first.BatchIDs[0], *snapshot.Items[0].BatchID)
}

func TestApproveDeliveryFullyWithTwoItemsOfOneProduct(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	registered := f.register(t, item(cream, 300, oct(20)), item(cream, 200, oct(12)))

	approved, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
	require.NoError(t, err)

	batches := f.batches(t, cream.ID)
	require.Len(t, batches, 2)
	// порядок FEFO: сначала партия с ближайшим сроком, хотя в поставке она вторая
	require.Equal(t, approved.Items[1].BatchIDs[0], batches[0].ID)
	require.Equal(t, oct(12), batches[0].ExpiresAt)
	require.Equal(t, oct(20), batches[1].ExpiresAt)
	require.Len(t, f.events(t, inventory.StockStream(point.ID, cream.ID)), 2)
}

func TestBatchesAreOrderedByExpiry(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	for _, day := range []int{20, 12, 15} {
		registered := f.register(t, item(cream, 100, oct(day)))
		_, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
		require.NoError(t, err)
	}

	batches := f.batches(t, cream.ID)

	require.Len(t, batches, 3)
	require.Equal(t, []time.Time{oct(12), oct(15), oct(20)},
		[]time.Time{batches[0].ExpiresAt, batches[1].ExpiresAt, batches[2].ExpiresAt})
}

func TestApproveDeliveryRejectedLeavesNoTrace(t *testing.T) {
	f := setup(t)
	cream, bacon := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil)
	registered := f.register(t, item(cream, 10, oct(15)), item(bacon, 5, oct(20)))
	before := f.totalEvents(t)

	invalid := map[string][]inventory.ItemDecision{
		"sum does not match": {
			{ItemID: registered.Items[0].ID, AcceptedQuantity: 6, RejectedQuantity: 3, RejectReason: "r", Comment: "c"},
			{ItemID: registered.Items[1].ID, AcceptedQuantity: 5},
		},
		"rejected without reason": {
			{ItemID: registered.Items[0].ID, AcceptedQuantity: 7, RejectedQuantity: 3},
			{ItemID: registered.Items[1].ID, AcceptedQuantity: 5},
		},
		"not all items decided": {{ItemID: registered.Items[0].ID, AcceptedQuantity: 10}},
	}
	for name, decisions := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, decisions)
			requireInvalid(t, err)
		})
	}

	read, err := f.svc.Delivery(f.ctx, registered.ID)
	require.NoError(t, err)
	require.Equal(t, inventory.DeliveryRegisteredStatus, read.Status)
	require.Empty(t, f.batches(t, cream.ID))
	require.Empty(t, f.batches(t, bacon.ID))
	require.Equal(t, before, f.totalEvents(t))
}

// Просрочка определяется по времени приёмки.

func TestItemExpiredBetweenRegistrationAndApproval(t *testing.T) {
	f := setup(t)
	cream, bacon := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil)
	registered := f.register(t, item(cream, 10, oct(11)), item(bacon, 5, oct(20)))
	before := f.totalEvents(t)

	f.clock.Set(oct(12)) // срок сливок истёк, пока поставка ждала приёмки

	_, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
	var validation *inventory.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Contains(t, validation.Field, registered.Items[0].ID.String())
	require.Equal(t, before, f.totalEvents(t))
	require.Empty(t, f.batches(t, bacon.ID), "approval is all or nothing")

	// та же позиция, отклонённая целиком, принимается с отметкой о просрочке
	approved, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, []inventory.ItemDecision{
		{ItemID: registered.Items[0].ID, RejectedQuantity: 10, RejectReason: "expired", Comment: "срок истёк"},
		{ItemID: registered.Items[1].ID, AcceptedQuantity: 5},
	})
	require.NoError(t, err)
	require.True(t, approved.Items[0].Expired)
	require.False(t, approved.Items[1].Expired)
	require.Empty(t, f.batches(t, cream.ID))
	require.Len(t, f.batches(t, bacon.ID), 1)
}

// Однократность и атомарность.

func TestApproveDeliveryOnlyOnce(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	registered := f.register(t, item(cream, 10, oct(15)))
	_, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
	require.NoError(t, err)
	before := f.totalEvents(t)

	_, err = f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))

	require.ErrorIs(t, err, inventory.ErrAlreadyApproved)
	require.Equal(t, before, f.totalEvents(t))
	require.Len(t, f.batches(t, cream.ID), 1)
}

func TestConcurrentApprovals(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	registered := f.register(t, item(cream, 10, oct(15)))

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
			errs <- err
		}()
	}
	first, second := <-errs, <-errs

	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, inventory.ErrAlreadyApproved)
	require.Len(t, f.batches(t, cream.ID), 1)
	require.Len(t, f.events(t, inventory.StockStream(point.ID, cream.ID)), 1)
	require.Len(t, f.events(t, inventory.DeliveryStream(registered.ID)), 2)
}

func TestApproveDeliveryIsIdempotent(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	registered := f.register(t, item(cream, 10, oct(15)))

	first, err := f.svc.ApproveDelivery(f.ctx, manager, "key-1", registered.ID, acceptAll(registered))
	require.NoError(t, err)
	before := f.totalEvents(t)
	second, err := f.svc.ApproveDelivery(f.ctx, manager, "key-1", registered.ID, acceptAll(registered))
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, before, f.totalEvents(t))
	require.Len(t, f.batches(t, cream.ID), 1)
}

func TestRegisterDeliveryIsIdempotent(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	in := inventory.NewDelivery{Number: "ПН-1", DeliveredAt: startOfDay, Items: []inventory.NewDeliveryItem{item(cream, 10, oct(15))}}

	first, err := f.svc.RegisterDelivery(f.ctx, manager, "key-1", point, in)
	require.NoError(t, err)
	before := f.totalEvents(t)
	second, err := f.svc.RegisterDelivery(f.ctx, manager, "key-1", point, in)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, before, f.totalEvents(t))
}

// Ограничение базы.

func TestBatchQuantityCannotGoNegative(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	registered := f.register(t, item(cream, 10, oct(15)))
	_, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
	require.NoError(t, err)

	_, err = f.pool.Exec(f.ctx, "UPDATE inventory_batches SET quantity = quantity - 11")

	require.ErrorContains(t, err, "inventory_batches_quantity_check")
	require.Equal(t, int64(10), f.batches(t, cream.ID)[0].Quantity)
}

func ptr[T any](v T) *T { return &v }

// Списание под заказ.

// consume вызывает ConsumeForOrder так, как это делает команда заказа: под блокировками складских потоков.
func (f fixture) consume(t *testing.T, locked []catalog.Product, needs ...inventory.Need) ([]inventory.Shortage, error) {
	t.Helper()
	locks := make([]string, len(locked))
	for i, p := range locked {
		locks[i] = inventory.StockStream(point.ID, p.ID)
	}
	spec := command.Spec{Name: "TestConsume", Actor: manager, Locks: locks}
	return command.Run(f.ctx, f.runner, spec, func(ctx context.Context, tx *command.Tx) ([]inventory.Shortage, error) {
		return f.svc.ConsumeForOrder(ctx, tx, point.ID, orderID, needs)
	})
}

func (f fixture) supply(t *testing.T, product catalog.Product, quantity int64, expiresAt time.Time) uuid.UUID {
	t.Helper()
	registered := f.register(t, item(product, quantity, expiresAt))
	approved, err := f.svc.ApproveDelivery(f.ctx, manager, "", registered.ID, acceptAll(registered))
	require.NoError(t, err)
	return approved.Items[0].BatchIDs[0]
}

func (f fixture) quantities(t *testing.T, product catalog.Product) []int64 {
	t.Helper()
	batches := f.batches(t, product.ID)
	out := make([]int64, len(batches))
	for i, b := range batches {
		out[i] = b.Quantity
	}
	return out
}

func TestConsumeForOrderFEFO(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	late := f.supply(t, cream, 100, oct(20))
	early := f.supply(t, cream, 100, oct(12))
	middle := f.supply(t, cream, 100, oct(15))

	shortages, err := f.consume(t, []catalog.Product{cream}, inventory.Need{ProductID: cream.ID, Quantity: 250})

	require.NoError(t, err)
	require.Empty(t, shortages)
	require.Equal(t, []int64{0, 0, 50}, f.quantities(t, cream))
	events := f.events(t, inventory.StockStream(point.ID, cream.ID))
	require.Equal(t, inventory.StockConsumed{PointID: point.ID, ProductID: cream.ID, OrderID: orderID, Batches: []inventory.BatchQuantity{
		{BatchID: early, Quantity: 100}, {BatchID: middle, Quantity: 100}, {BatchID: late, Quantity: 50},
	}}, events[len(events)-1])
	// исходное количество партии не меняется, меняется только остаток
	require.Equal(t, int64(100), f.batches(t, cream.ID)[2].ReceivedQuantity)
}

func TestConsumeForOrderIsAllOrNothing(t *testing.T) {
	f := setup(t)
	cream, bacon := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil)
	f.supply(t, cream, 100, oct(20))
	f.supply(t, bacon, 50, oct(20))
	before := f.totalEvents(t)

	shortages, err := f.consume(t, []catalog.Product{cream, bacon},
		inventory.Need{ProductID: cream.ID, Quantity: 30},
		inventory.Need{ProductID: bacon.ID, Quantity: 120})

	require.NoError(t, err)
	require.Equal(t, []inventory.Shortage{{ProductID: bacon.ID, Required: 120, Available: 50}}, shortages)
	require.Equal(t, []int64{100}, f.quantities(t, cream), "the product with enough stock is not touched either")
	require.Equal(t, []int64{50}, f.quantities(t, bacon))
	require.Equal(t, before, f.totalEvents(t))
}

func TestConsumeForOrderIgnoresExpiredBatches(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	f.supply(t, cream, 100, oct(12))
	f.supply(t, cream, 100, oct(15))
	f.clock.Set(oct(13))

	shortages, err := f.consume(t, []catalog.Product{cream}, inventory.Need{ProductID: cream.ID, Quantity: 150})

	require.NoError(t, err)
	require.Equal(t, []inventory.Shortage{{ProductID: cream.ID, Required: 150, Available: 100}}, shortages)

	shortages, err = f.consume(t, []catalog.Product{cream}, inventory.Need{ProductID: cream.ID, Quantity: 60})
	require.NoError(t, err)
	require.Empty(t, shortages)
	require.Equal(t, []int64{100, 40}, f.quantities(t, cream), "expired batch keeps its quantity")
}

func TestConsumeForOrderRequiresLock(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	f.supply(t, cream, 100, oct(20))

	_, err := f.consume(t, nil, inventory.Need{ProductID: cream.ID, Quantity: 10})

	require.ErrorContains(t, err, "without a lock")
	require.Equal(t, []int64{100}, f.quantities(t, cream))
}
