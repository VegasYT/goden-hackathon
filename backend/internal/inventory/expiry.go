package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory/db"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
)

// WorkerActor - автор автоматических складских операций. Worker вызывает сервис напрямую,
// а не через API, поэтому учётной записи Keycloak у него нет.
var WorkerActor = eventstore.Actor{Type: eventstore.ActorService, ID: "worker"}

// сколько партий каждого вида обрабатывается за один проход; остальные - в следующем
const expiryBatchLimit = 500

// ExpiryChecker - фоновая проверка сроков годности: отмечает просроченные партии и выдаёт
// предупреждения по истекающим. Остаток она не меняет: просрочку списывает вручную управляющий.
//
// Запросы только находят кандидатов. Решение принимают команды: они перечитывают партию
// под блокировкой и ничего не делают, если делать уже нечего. Поэтому несколько экземпляров
// процесса и гонка с заказом безопасны без дополнительной координации.
type ExpiryChecker struct {
	svc    *Service
	logger *slog.Logger
	// WarningPeriod - за сколько до окончания срока годности выдаётся предупреждение.
	warningPeriod time.Duration
	interval      time.Duration

	markExpired func(ctx context.Context, batchID uuid.UUID) (bool, error)
	warn        func(ctx context.Context, batchID uuid.UUID) (bool, error)

	expiring prometheus.Gauge
	expired  prometheus.Gauge
	checks   *prometheus.CounterVec
}

// NewExpiryChecker создаёт проверку и регистрирует её метрики в реестре процесса.
func NewExpiryChecker(svc *Service, logger *slog.Logger, warningPeriod, interval time.Duration) *ExpiryChecker {
	registry := svc.runner.Registry()
	c := &ExpiryChecker{
		svc: svc, logger: logger, warningPeriod: warningPeriod, interval: interval,
		expiring: observability.Register(registry, prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: observability.MetricsNamespace, Name: "batches_expiring",
			Help: "Batches with stock that expire within the warning period, as of the last check.",
		})),
		expired: observability.Register(registry, prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: observability.MetricsNamespace, Name: "batches_expired",
			Help: "Expired batches that still have stock, as of the last check.",
		})),
		checks: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "expiry_checks_total",
			Help: "Passes of the expiry check by outcome: ok, error (at least one batch failed).",
		}, []string{"outcome"})),
	}
	c.markExpired = func(ctx context.Context, batchID uuid.UUID) (bool, error) {
		return svc.MarkBatchExpired(ctx, WorkerActor, batchID)
	}
	c.warn = func(ctx context.Context, batchID uuid.UUID) (bool, error) {
		return svc.IssueBatchExpiryWarning(ctx, WorkerActor, batchID, warningPeriod)
	}
	for _, outcome := range []string{"ok", "error"} {
		c.checks.WithLabelValues(outcome) // серии с нулём, чтобы первый проход был виден в increase()
	}
	return c
}

// Run выполняет проверку раз в interval, пока не отменён ctx. Ошибка прохода не завершает работу.
func (c *ExpiryChecker) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		if err := c.Check(ctx); err != nil && ctx.Err() == nil {
			c.logger.ErrorContext(ctx, "expiry check failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Check выполняет один проход: сначала отмечает просрочку, затем выдаёт предупреждения.
// Ошибка по одной партии не прерывает проход: партия будет обработана в следующем.
func (c *ExpiryChecker) Check(ctx context.Context) error {
	q := db.New(c.svc.pool)
	now := c.svc.runner.Now()
	until := now.Add(c.warningPeriod)

	expired, err := q.ListExpiredBatchIDs(ctx, db.ListExpiredBatchIDsParams{Now: now, MaxRows: expiryBatchLimit})
	if err != nil {
		c.checks.WithLabelValues("error").Inc()
		return fmt.Errorf("inventory: find expired batches: %w", err)
	}
	toWarn, err := q.ListBatchIDsToWarn(ctx, db.ListBatchIDsToWarnParams{Now: now, Until: until, MaxRows: expiryBatchLimit})
	if err != nil {
		c.checks.WithLabelValues("error").Inc()
		return fmt.Errorf("inventory: find batches to warn about: %w", err)
	}

	failed := 0
	for _, id := range expired {
		if _, err := c.markExpired(ctx, id); err != nil {
			failed++
			c.logger.ErrorContext(ctx, "expired batch is not marked", "batch_id", id, "error", err)
		}
	}
	for _, id := range toWarn {
		if _, err := c.warn(ctx, id); err != nil {
			failed++
			c.logger.ErrorContext(ctx, "expiry warning is not issued", "batch_id", id, "error", err)
		}
	}

	// датчики - после обработки; просроченные партии с остатком ждут ручного списания
	counts, err := q.CountExpiryState(ctx, db.CountExpiryStateParams{Now: now, Until: until})
	if err == nil {
		c.expiring.Set(float64(counts.Expiring))
		c.expired.Set(float64(counts.Expired))
	}
	if failed > 0 || err != nil {
		c.checks.WithLabelValues("error").Inc()
		return fmt.Errorf("inventory: expiry check: %d batches failed", failed)
	}
	c.checks.WithLabelValues("ok").Inc()
	return nil
}

// ExpiringBatch - партия с остатком, срок годности которой истёк или скоро истечёт.
type ExpiringBatch struct {
	Batch   Batch
	Product catalog.Product
	// Expired - срок годности истёк на момент запроса.
	Expired bool
}

// ExpiringBatches возвращает страницу партий точки с остатком, срок годности которых истёк или истекает
// в пределах within, от ближайшего срока, и их общее число. Читается из таблицы партий:
// принадлежность к истекающим зависит от текущего времени.
func (s *Service) ExpiringBatches(ctx context.Context, pointID uuid.UUID, within time.Duration, page catalog.Page) ([]ExpiringBatch, int64, error) {
	q := db.New(s.pool)
	now := s.runner.Now()
	until := now.Add(within)

	rows, err := q.ListExpiringBatches(ctx, db.ListExpiringBatchesParams{PointID: pointID, Until: until, PageLimit: page.Limit, PageOffset: page.Offset})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: list expiring batches: %w", err)
	}
	total, err := q.CountExpiringBatches(ctx, db.CountExpiringBatchesParams{PointID: pointID, Until: until})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: count expiring batches: %w", err)
	}

	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ProductID
	}
	products, err := s.products.ProductsByID(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	batches := make([]ExpiringBatch, len(rows))
	for i, row := range rows {
		batch := batchFromRow(row)
		batches[i] = ExpiringBatch{Batch: batch, Product: products[batch.ProductID], Expired: batch.expiredAt(now)}
	}
	return batches, total, nil
}
