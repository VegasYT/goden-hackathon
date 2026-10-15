//go:build integration

package inventory_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/eventstore"
)

var worker = eventstore.Actor{Type: eventstore.ActorService, ID: "worker"}

func (f fixture) batch(t *testing.T, id uuid.UUID) inventory.Batch {
	t.Helper()
	b, err := f.svc.Batch(f.ctx, id)
	require.NoError(t, err)
	return b
}

func (f fixture) lastStockEvent(t *testing.T, product catalog.Product) eventstore.Payload {
	t.Helper()
	events := f.events(t, inventory.StockStream(point.ID, product.ID))
	return events[len(events)-1]
}

// Ручное списание.

func TestWriteOffStock(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 1000, oct(20))

	after, err := f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
		BatchID: batchID, Quantity: 300, Reason: inventory.WriteOffStorageViolation, Comment: " оставили вне холодильника ",
	})
	require.NoError(t, err)

	require.Equal(t, int64(700), after.Quantity)
	require.Equal(t, int64(1000), after.ReceivedQuantity)
	require.Equal(t, after, f.batch(t, batchID))
	require.Equal(t, inventory.StockWrittenOff{
		PointID: point.ID, ProductID: cream.ID, BatchID: batchID, Quantity: 300,
		Reason: inventory.WriteOffStorageViolation, Comment: "оставили вне холодильника", WrittenOffBy: manager,
	}, f.lastStockEvent(t, cream))

	// списание всего остатка
	after, err = f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
		BatchID: batchID, Quantity: 700, Reason: inventory.WriteOffPhysicalDamage, Comment: "упала коробка",
	})
	require.NoError(t, err)
	require.Zero(t, after.Quantity)
}

func TestWriteOffStockRejected(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 100, oct(20))
	before := f.totalEvents(t)

	invalid := map[string]inventory.WriteOff{
		"more than the batch has":       {BatchID: batchID, Quantity: 150, Reason: inventory.WriteOffOther, Comment: "c"},
		"no comment":                    {BatchID: batchID, Quantity: 10, Reason: inventory.WriteOffOther},
		"unknown reason":                {BatchID: batchID, Quantity: 10, Reason: "lost", Comment: "c"},
		"reason expired for good batch": {BatchID: batchID, Quantity: 10, Reason: inventory.WriteOffExpired, Comment: "c"},
		"zero quantity":                 {BatchID: batchID, Reason: inventory.WriteOffOther, Comment: "c"},
	}
	for name, in := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.WriteOffStock(f.ctx, manager, "", in)
			requireInvalid(t, err)
		})
	}
	t.Run("unknown batch", func(t *testing.T) {
		_, err := f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
			BatchID: uuid.Must(uuid.NewV7()), Quantity: 1, Reason: inventory.WriteOffOther, Comment: "c",
		})
		require.ErrorIs(t, err, inventory.ErrNotFound)
	})

	require.Equal(t, int64(100), f.batch(t, batchID).Quantity)
	require.Equal(t, before, f.totalEvents(t))
}

func TestWriteOffStockIsIdempotent(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 1000, oct(20))
	in := inventory.WriteOff{BatchID: batchID, Quantity: 300, Reason: inventory.WriteOffQualityIssue, Comment: "запах"}

	first, err := f.svc.WriteOffStock(f.ctx, manager, "key-1", in)
	require.NoError(t, err)
	before := f.totalEvents(t)
	second, err := f.svc.WriteOffStock(f.ctx, manager, "key-1", in)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, int64(700), f.batch(t, batchID).Quantity)
	require.Equal(t, before, f.totalEvents(t))
}

// Просрочка: отметка без списания, списывает человек.

func TestMarkBatchExpired(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 400, oct(12))

	// срок ещё не истёк
	done, err := f.svc.MarkBatchExpired(f.ctx, worker, batchID)
	require.NoError(t, err)
	require.False(t, done)

	f.clock.Set(oct(12))
	before := f.totalEvents(t)

	done, err = f.svc.MarkBatchExpired(f.ctx, worker, batchID)
	require.NoError(t, err)
	require.True(t, done)
	require.Equal(t, int64(400), f.batch(t, batchID).Quantity, "the notice does not change the stock")
	require.Equal(t, inventory.BatchExpired{
		PointID: point.ID, ProductID: cream.ID, BatchID: batchID, RemainingQuantity: 400,
		ExpiresAt: eventstore.At(oct(12)),
	}, f.lastStockEvent(t, cream))
	require.Equal(t, before+1, f.totalEvents(t))

	// повторный вызов ничего не делает
	done, err = f.svc.MarkBatchExpired(f.ctx, worker, batchID)
	require.NoError(t, err)
	require.False(t, done)
	require.Equal(t, before+1, f.totalEvents(t))

	// списывает управляющий
	after, err := f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
		BatchID: batchID, Quantity: 400, Reason: inventory.WriteOffExpired, Comment: "просрочка",
	})
	require.NoError(t, err)
	require.Zero(t, after.Quantity)

	_, err = f.svc.MarkBatchExpired(f.ctx, worker, uuid.Must(uuid.NewV7()))
	require.ErrorIs(t, err, inventory.ErrNotFound)
}

// Отметка просрочки и заказы по тому же продукту идут по очереди, заказы не трогают
// просроченную партию, её остаток не меняется.
func TestExpiredNoticeRacesWithOrders(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	expired := f.supply(t, cream, 100, oct(12))
	good := f.supply(t, cream, 1000, oct(20))
	f.clock.Set(oct(13))

	var g errgroup.Group
	g.Go(func() error {
		_, err := f.svc.MarkBatchExpired(f.ctx, worker, expired)
		return err
	})
	for range 8 {
		g.Go(func() error {
			shortages, err := f.consume(t, []catalog.Product{cream}, inventory.Need{ProductID: cream.ID, Quantity: 50})
			if err == nil && len(shortages) > 0 {
				t.Errorf("unexpected shortage: %+v", shortages)
			}
			return err
		})
	}
	require.NoError(t, g.Wait())

	require.Equal(t, int64(100), f.batch(t, expired).Quantity)
	require.Equal(t, int64(1000-8*50), f.batch(t, good).Quantity)

	var notices []inventory.BatchExpired
	for _, event := range f.events(t, inventory.StockStream(point.ID, cream.ID)) {
		switch e := event.(type) {
		case inventory.BatchExpired:
			notices = append(notices, e)
		case inventory.StockConsumed:
			for _, taken := range e.Batches {
				require.Equal(t, good, taken.BatchID, "orders must not consume the expired batch")
			}
		}
	}
	require.Len(t, notices, 1)
	require.Equal(t, int64(100), notices[0].RemainingQuantity)

	// читатели получают события складского потока в порядке версий
	all, err := eventstore.ReadAll(f.ctx, f.pool, eventstore.Checkpoint{}, 1000)
	require.NoError(t, err)
	var lastVersion int64
	for _, e := range all {
		if e.StreamID == inventory.StockStream(point.ID, cream.ID) {
			require.Equal(t, lastVersion+1, e.Version)
			lastVersion = e.Version
		}
	}
	require.Equal(t, int64(2+1+8), lastVersion)
}

// Корректировка.

func TestAdjustStock(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 1000, oct(20))

	after, err := f.svc.AdjustStock(f.ctx, manager, "", inventory.Adjustment{
		BatchID: batchID, Delta: -50, Reason: "inventory_count", Comment: "пересчёт",
	})
	require.NoError(t, err)
	require.Equal(t, int64(950), after.Quantity)
	require.Equal(t, inventory.StockAdjusted{
		PointID: point.ID, ProductID: cream.ID, BatchID: batchID, Delta: -50,
		Reason: "inventory_count", Comment: "пересчёт", AdjustedBy: manager,
	}, f.lastStockEvent(t, cream))

	after, err = f.svc.AdjustStock(f.ctx, manager, "", inventory.Adjustment{BatchID: batchID, Delta: 70, Reason: "inventory_count"})
	require.NoError(t, err)
	require.Equal(t, int64(1020), after.Quantity)
	require.Equal(t, after, f.batch(t, batchID))
}

func TestAdjustStockRejected(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 100, oct(20))
	before := f.totalEvents(t)

	invalid := map[string]inventory.Adjustment{
		"below zero": {BatchID: batchID, Delta: -150, Reason: "inventory_count"},
		"zero delta": {BatchID: batchID, Reason: "inventory_count"},
		"no reason":  {BatchID: batchID, Delta: -1},
	}
	for name, in := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.AdjustStock(f.ctx, manager, "", in)
			requireInvalid(t, err)
		})
	}
	t.Run("unknown batch", func(t *testing.T) {
		_, err := f.svc.AdjustStock(f.ctx, manager, "", inventory.Adjustment{
			BatchID: uuid.Must(uuid.NewV7()), Delta: 1, Reason: "inventory_count",
		})
		require.ErrorIs(t, err, inventory.ErrNotFound)
	})

	require.Equal(t, int64(100), f.batch(t, batchID).Quantity)
	require.Equal(t, before, f.totalEvents(t))
}

// Предупреждение о сроке.

func TestIssueBatchExpiryWarning(t *testing.T) {
	const day = 24 * time.Hour
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 1200, oct(15))

	// до срока почти 5 дней: рано
	issued, err := f.svc.IssueBatchExpiryWarning(f.ctx, worker, batchID, day)
	require.NoError(t, err)
	require.False(t, issued)
	require.Nil(t, f.batch(t, batchID).ExpiryWarnedAt)

	warnedAt := oct(14).Add(4 * time.Hour) // до срока 20 часов
	f.clock.Set(warnedAt)
	before := f.totalEvents(t)

	issued, err = f.svc.IssueBatchExpiryWarning(f.ctx, worker, batchID, day)
	require.NoError(t, err)
	require.True(t, issued)
	require.Equal(t, inventory.BatchExpiryWarningIssued{
		PointID: point.ID, ProductID: cream.ID, BatchID: batchID, RemainingQuantity: 1200,
		ExpiresAt: eventstore.At(oct(15)), WarningPeriodSeconds: 86400,
	}, f.lastStockEvent(t, cream))
	after := f.batch(t, batchID)
	require.Equal(t, int64(1200), after.Quantity, "a warning does not change stock")
	require.Equal(t, &warnedAt, after.ExpiryWarnedAt)

	// один раз на партию, даже с другим периодом
	for _, period := range []time.Duration{day, 2 * day} {
		issued, err = f.svc.IssueBatchExpiryWarning(f.ctx, worker, batchID, period)
		require.NoError(t, err)
		require.False(t, issued)
	}
	require.Equal(t, before+1, f.totalEvents(t))
}

func TestIssueBatchExpiryWarningSkipsExpiredAndEmpty(t *testing.T) {
	const day = 24 * time.Hour
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	expired := f.supply(t, cream, 100, oct(12))
	empty := f.supply(t, cream, 100, oct(14))
	_, err := f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
		BatchID: empty, Quantity: 100, Reason: inventory.WriteOffOther, Comment: "c",
	})
	require.NoError(t, err)
	f.clock.Set(oct(13))
	before := f.totalEvents(t)

	for _, batchID := range []uuid.UUID{expired, empty} {
		issued, err := f.svc.IssueBatchExpiryWarning(f.ctx, worker, batchID, day)
		require.NoError(t, err)
		require.False(t, issued)
	}
	require.Equal(t, before, f.totalEvents(t))

	_, err = f.svc.IssueBatchExpiryWarning(f.ctx, worker, expired, 0)
	requireInvalid(t, err)
	_, err = f.svc.IssueBatchExpiryWarning(f.ctx, worker, uuid.Must(uuid.NewV7()), day)
	require.ErrorIs(t, err, inventory.ErrNotFound)
}
