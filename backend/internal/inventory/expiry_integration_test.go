//go:build integration

package inventory_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability/metricstest"
)

const warningPeriod = 72 * time.Hour

func (f fixture) checker() *inventory.ExpiryChecker {
	return inventory.NewExpiryChecker(f.svc, slog.New(slog.DiscardHandler), warningPeriod, time.Minute)
}

// stockEvents возвращает события складского потока продукта заданного типа.
func stockEvents[T eventstore.Payload](t *testing.T, f fixture, productID uuid.UUID) []T {
	t.Helper()
	var out []T
	for _, payload := range f.events(t, inventory.StockStream(point.ID, productID)) {
		if e, ok := payload.(T); ok {
			out = append(out, e)
		}
	}
	return out
}

// expiredOnList возвращает партии из списка истекающих, отмеченные просроченными.
func expiredOnList(t *testing.T, f fixture) []uuid.UUID {
	t.Helper()
	batches, _, err := f.svc.ExpiringBatches(f.ctx, point.ID, warningPeriod, catalog.Page{Limit: 100})
	require.NoError(t, err)
	var out []uuid.UUID
	for _, b := range batches {
		if b.Expired {
			out = append(out, b.Batch.ID)
		}
	}
	return out
}

func TestExpiryCheckMarksExpiredBatches(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	expiring := f.supply(t, cream, 400, oct(12))
	f.supply(t, cream, 250, oct(25))
	c := f.checker()

	// срок ещё не истёк: отметки нет
	require.NoError(t, c.Check(f.ctx))
	require.Empty(t, stockEvents[inventory.BatchExpired](t, f, cream.ID))

	f.clock.Set(oct(12))
	require.NoError(t, c.Check(f.ctx))
	require.Equal(t, []int64{400, 250}, f.quantities(t, cream), "the check does not write off")
	notices := stockEvents[inventory.BatchExpired](t, f, cream.ID)
	require.Len(t, notices, 1)
	require.Equal(t, expiring, notices[0].BatchID)
	require.Equal(t, int64(400), notices[0].RemainingQuantity)
	require.Empty(t, stockEvents[inventory.StockWrittenOff](t, f, cream.ID))

	// повторные проходы ничего не добавляют, партия ждёт решения в списке
	before := f.totalEvents(t)
	for range 3 {
		require.NoError(t, c.Check(f.ctx))
	}
	require.Equal(t, before, f.totalEvents(t))
	require.Equal(t, []uuid.UUID{expiring}, expiredOnList(t, f))

	// управляющий списал: партии нет в списке, проверка не добавляет событий
	_, err := f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
		BatchID: expiring, Quantity: 400, Reason: inventory.WriteOffExpired, Comment: "просрочка",
	})
	require.NoError(t, err)
	require.Empty(t, expiredOnList(t, f))
	before = f.totalEvents(t)
	require.NoError(t, c.Check(f.ctx))
	require.Equal(t, before, f.totalEvents(t))
}

func TestExpiryCheckWarnsOnce(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 1200, oct(15))
	c := f.checker()

	// до срока больше периода предупреждения
	require.NoError(t, c.Check(f.ctx))
	require.Empty(t, stockEvents[inventory.BatchExpiryWarningIssued](t, f, cream.ID))

	// время дошло до периода
	f.clock.Set(oct(13))
	for range 3 {
		require.NoError(t, c.Check(f.ctx))
	}
	warnings := stockEvents[inventory.BatchExpiryWarningIssued](t, f, cream.ID)
	require.Len(t, warnings, 1, "one warning however many times the check runs")
	require.Equal(t, batchID, warnings[0].BatchID)
	require.Equal(t, int64(1200), warnings[0].RemainingQuantity)
	require.Equal(t, int64(warningPeriod.Seconds()), warnings[0].WarningPeriodSeconds)
	require.Equal(t, []int64{1200}, f.quantities(t, cream), "a warning does not change stock")

	// срок истёк: партия отмечена, остаток не тронут, предупреждение осталось одно
	f.clock.Set(oct(15))
	require.NoError(t, c.Check(f.ctx))
	require.Equal(t, []int64{1200}, f.quantities(t, cream))
	require.Len(t, stockEvents[inventory.BatchExpiryWarningIssued](t, f, cream.ID), 1)
	require.Len(t, stockEvents[inventory.BatchExpired](t, f, cream.ID), 1)
}

// Пустая партия не интересует проверку: ни предупреждения, ни отметки о просрочке.
func TestExpiryCheckIgnoresEmptyBatches(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	batchID := f.supply(t, cream, 100, oct(12))
	_, err := f.svc.WriteOffStock(f.ctx, manager, "", inventory.WriteOff{
		BatchID: batchID, Quantity: 100, Reason: inventory.WriteOffPhysicalDamage, Comment: "разбили",
	})
	require.NoError(t, err)
	before := f.totalEvents(t)

	f.clock.Set(oct(11))
	require.NoError(t, f.checker().Check(f.ctx))
	f.clock.Set(oct(13))
	require.NoError(t, f.checker().Check(f.ctx))
	require.Equal(t, before, f.totalEvents(t))
}

func TestConcurrentExpiryChecks(t *testing.T) {
	f := setup(t)
	cream, bacon := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil)
	f.supply(t, cream, 400, oct(12))
	f.supply(t, bacon, 300, oct(14))
	f.clock.Set(oct(12))

	// несколько экземпляров worker проверяют одни и те же партии одновременно
	var g errgroup.Group
	for range 6 {
		c := f.checker()
		g.Go(func() error { return c.Check(f.ctx) })
	}
	require.NoError(t, g.Wait())

	require.Len(t, stockEvents[inventory.BatchExpired](t, f, cream.ID), 1)
	require.Len(t, stockEvents[inventory.BatchExpiryWarningIssued](t, f, bacon.ID), 1)
	require.Equal(t, []int64{400}, f.quantities(t, cream))
}

func TestExpiryCheckSurvivesAFailedBatch(t *testing.T) {
	f := setup(t)
	cream, bacon, eggs := f.product(t, "Сливки", nil), f.product(t, "Бекон", nil), f.product(t, "Яйцо", nil)
	f.supply(t, cream, 100, oct(12))
	broken := f.supply(t, bacon, 200, oct(12))
	f.supply(t, eggs, 300, oct(12))
	f.clock.Set(oct(13))
	c := f.checker()
	registry := f.runner.Registry()

	notices := func(p catalog.Product) int { return len(stockEvents[inventory.BatchExpired](t, f, p.ID)) }

	c.FailOn(&broken)
	require.Error(t, c.Check(f.ctx))
	require.Equal(t, 1, notices(cream))
	require.Zero(t, notices(bacon), "the failed batch is left for the next pass")
	require.Equal(t, 1, notices(eggs), "the batches after the failed one are processed")
	require.EqualValues(t, 1, metricstest.Value(t, registry, "goden_expiry_checks_total", "outcome", "error"))
	require.EqualValues(t, 3, metricstest.Value(t, registry, "goden_batches_expired"), "expired batches wait for a write-off")

	// причина устранена: следующий проход дописывает
	healthy := f.checker()
	require.NoError(t, healthy.Check(f.ctx))
	require.Equal(t, 1, notices(bacon))
	require.EqualValues(t, 1, metricstest.Value(t, registry, "goden_expiry_checks_total", "outcome", "ok"))
}

func TestExpiryMetrics(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	f.supply(t, cream, 100, oct(12)) // истекает в периоде предупреждения
	f.supply(t, cream, 100, oct(25)) // до срока далеко
	registry := f.runner.Registry()

	require.NoError(t, f.checker().Check(f.ctx))
	require.EqualValues(t, 1, metricstest.Value(t, registry, "goden_batches_expiring"))
	require.Zero(t, metricstest.Value(t, registry, "goden_batches_expired"))
}

func TestExpiryCheckerRunStopsOnCancel(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", nil)
	f.supply(t, cream, 100, oct(12))
	f.clock.Set(oct(12))

	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan error, 1)
	c := inventory.NewExpiryChecker(f.svc, slog.New(slog.DiscardHandler), warningPeriod, 10*time.Millisecond)
	go func() { done <- c.Run(ctx) }()

	require.Eventually(t, func() bool {
		return len(stockEvents[inventory.BatchExpired](t, f, cream.ID)) == 1
	}, 10*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}
