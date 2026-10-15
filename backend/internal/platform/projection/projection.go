// Package projection - движок асинхронных проекций Event Store (ADR-002).
//
// Проекция получает все события в общем порядке чтения, каждое ровно один раз:
// порция событий применяется в одной транзакции с сохранением позиции. Автор проекции
// пишет только Apply и Reset; о пропусках, повторах, перезапусках и втором экземпляре
// процесса заботится движок.
package projection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
)

const (
	defaultBatchSize    = 200
	defaultPollInterval = 200 * time.Millisecond
	minBackoff          = 100 * time.Millisecond
	maxBackoff          = 5 * time.Second
)

// Projection - асинхронная read-модель.
type Projection struct {
	// Name - ключ позиции и метка метрик. У каждой проекции своё.
	Name string
	// Version - версия логики и схемы таблиц проекции. Если она отличается от сохранённой,
	// движок очищает проекцию и применяет все события заново. Увеличивается при любом
	// изменении, после которого старые данные проекции неверны.
	Version int
	// Apply применяет событие в транзакции движка. Вызывается для каждого события Event Store;
	// ненужные события проекция пропускает сама. Меняет только таблицы проекции и не обращается
	// к сети: транзакция держит блокировку позиции.
	Apply func(ctx context.Context, tx pgx.Tx, e eventstore.Event) error
	// Reset очищает таблицы проекции перед перестроением.
	Reset func(ctx context.Context, tx pgx.Tx) error
}

// CheckpointState идентифицирует содержимое асинхронной read-модели.
// Version меняется при перестроении логики, Checkpoint — после применённой порции событий.
type CheckpointState struct {
	Version int
	eventstore.Checkpoint
}

// ReadCheckpoint возвращает сохранённую версию и позицию проекции без блокировки.
// found=false означает, что worker ещё не запускал эту проекцию.
func ReadCheckpoint(ctx context.Context, q eventstore.Querier, name string) (state CheckpointState, found bool, err error) {
	version, checkpoint, found, err := readCheckpoint(ctx, q, name, false)
	if err != nil {
		return CheckpointState{}, false, err
	}
	return CheckpointState{Version: version, Checkpoint: checkpoint}, found, nil
}

func (p Projection) validate() error {
	switch {
	case p.Name == "":
		return errors.New("projection: name is empty")
	case p.Apply == nil || p.Reset == nil:
		return fmt.Errorf("projection %s: Apply and Reset are required", p.Name)
	}
	return nil
}

// Engine выполняет проекции одной базы данных.
type Engine struct {
	pool   *pgxpool.Pool
	clock  clock.Clock
	logger *slog.Logger

	// BatchSize - сколько событий применяется в одной транзакции.
	BatchSize int
	// PollInterval - пауза между проверками, когда новых событий нет.
	PollInterval time.Duration

	lag         *prometheus.GaugeVec
	applied     *prometheus.CounterVec
	failures    *prometheus.CounterVec
	lastSuccess *prometheus.GaugeVec
}

// NewEngine создаёт движок и регистрирует метрики проекций в реестре процесса.
func NewEngine(pool *pgxpool.Pool, clk clock.Clock, logger *slog.Logger, registry prometheus.Registerer) *Engine {
	labels := []string{"projection"}
	return &Engine{
		pool: pool, clock: clk, logger: logger,
		BatchSize: defaultBatchSize, PollInterval: defaultPollInterval,
		lag: observability.Register(registry, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: observability.MetricsNamespace, Name: "projection_lag_events",
			Help: "Events of the Event Store the projection has not applied yet, including the ones that are not readable until older transactions finish.",
		}, labels)),
		applied: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "projection_events_total",
			Help: "Events applied by the projection.",
		}, labels)),
		failures: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "projection_errors_total",
			Help: "Failed batches of the projection; a failed batch is retried.",
		}, labels)),
		lastSuccess: observability.Register(registry, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: observability.MetricsNamespace, Name: "projection_last_success_timestamp_seconds",
			Help: "Unix time of the last successful step of the projection, with or without new events.",
		}, labels)),
	}
}

// Run применяет события к проекции, пока не отменён ctx. Ошибка применения не завершает работу:
// порция откатывается и повторяется с нарастающей паузой. Событие, которое проекция не может
// применить, останавливает её продвижение, а не пропускается.
func (e *Engine) Run(ctx context.Context, p Projection) error {
	if err := p.validate(); err != nil {
		return err
	}
	e.failures.WithLabelValues(p.Name) // серия с нулём: иначе первая ошибка не видна в increase()

	var backoff time.Duration
	for {
		n, err := e.step(ctx, p)
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			e.failures.WithLabelValues(p.Name).Inc()
			backoff = min(max(backoff*2, minBackoff), maxBackoff)
			e.logger.ErrorContext(ctx, "projection step failed", "projection", p.Name, "retry_in", backoff.String(), "error", err)
			e.observeLag(ctx, p)
			if !sleep(ctx, backoff) {
				return nil
			}
		case n < e.BatchSize:
			backoff = 0
			if !sleep(ctx, e.PollInterval) {
				return nil
			}
		default:
			backoff = 0 // порция полная: следующая читается сразу
		}
	}
}

// CatchUp применяет события, пока за позицией проекции не останется ни одного, и возвращается.
// Нужен тестам: после него данные проекции соответствуют всем зафиксированным событиям.
//
// Событие бывает зафиксировано, но ещё не доступно для чтения: его задерживает любая более
// ранняя незавершённая транзакция в кластере, в том числе служебная. Такие события CatchUp
// дожидается. Пока в Event Store непрерывно пишут, он не вернётся: ограничивай ожидание через ctx.
func (e *Engine) CatchUp(ctx context.Context, p Projection) error {
	if err := p.validate(); err != nil {
		return err
	}
	for {
		n, err := e.step(ctx, p)
		if err != nil {
			return err
		}
		if n == e.BatchSize {
			continue
		}
		remaining, err := e.remaining(ctx, p)
		if err != nil {
			return err
		}
		if remaining == 0 {
			return nil
		}
		if !sleep(ctx, e.PollInterval) {
			return ctx.Err()
		}
	}
}

// step выполняет одну порцию и возвращает число применённых событий.
func (e *Engine) step(ctx context.Context, p Projection) (int, error) {
	// Проверка без транзакции: пока событий нет, опрос не блокирует строку позиции
	// и не расходует номера транзакций.
	idle, err := e.idle(ctx, p)
	if err != nil {
		return 0, err
	}
	applied := 0
	if !idle {
		if applied, err = e.batch(ctx, p); err != nil {
			return 0, err
		}
	}
	e.applied.WithLabelValues(p.Name).Add(float64(applied))
	e.lastSuccess.WithLabelValues(p.Name).Set(float64(e.clock.Now().Unix()))
	e.observeLag(ctx, p)
	return applied, nil
}

// idle сообщает, что проекции нечего делать: версия совпадает и новых событий для чтения нет.
func (e *Engine) idle(ctx context.Context, p Projection) (bool, error) {
	version, after, found, err := readCheckpoint(ctx, e.pool, p.Name, false)
	if err != nil {
		return false, err
	}
	if !found || version != p.Version {
		return false, nil
	}
	events, err := eventstore.ReadAll(ctx, e.pool, after, 1)
	if err != nil {
		return false, err
	}
	return len(events) == 0, nil
}

// batch применяет порцию событий и сдвигает позицию в одной транзакции.
func (e *Engine) batch(ctx context.Context, p Projection) (int, error) {
	applied := 0
	err := pgx.BeginFunc(ctx, e.pool, func(tx pgx.Tx) error {
		now := e.clock.Now()
		_, err := tx.Exec(ctx, `
			INSERT INTO projection_checkpoints (name, version, updated_at) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, p.Name, p.Version, now)
		if err != nil {
			return fmt.Errorf("projection %s: create checkpoint: %w", p.Name, err)
		}
		// Блокировка строки: второй экземпляр процесса ждёт здесь и затем читает уже сдвинутую позицию.
		version, after, _, err := readCheckpoint(ctx, tx, p.Name, true)
		if err != nil {
			return err
		}
		if version != p.Version {
			// Перестроение: дальше оно идёт обычными порциями, эта транзакция только начинает его.
			if err := p.Reset(ctx, tx); err != nil {
				return fmt.Errorf("projection %s: reset: %w", p.Name, err)
			}
			after = eventstore.Checkpoint{}
			e.logger.InfoContext(ctx, "projection rebuild started", "projection", p.Name, "from_version", version, "to_version", p.Version)
		}

		events, err := eventstore.ReadAll(ctx, tx, after, e.BatchSize)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := p.Apply(ctx, tx, event); err != nil {
				return fmt.Errorf("projection %s: apply %s at position %d: %w", p.Name, event.Type, event.Position, err)
			}
			after = event.Checkpoint()
		}
		applied = len(events)

		_, err = tx.Exec(ctx, `
			UPDATE projection_checkpoints SET version = $2, txid = $3::text::xid8, position = $4, updated_at = $5
			WHERE name = $1`, p.Name, p.Version, strconv.FormatUint(after.TxID, 10), after.Position, now)
		if err != nil {
			return fmt.Errorf("projection %s: save checkpoint: %w", p.Name, err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return applied, nil
}

// readCheckpoint читает версию и позицию проекции; с lock блокирует строку до конца транзакции.
func readCheckpoint(ctx context.Context, q eventstore.Querier, name string, lock bool) (int, eventstore.Checkpoint, bool, error) {
	query := `SELECT version, txid::text, position FROM projection_checkpoints WHERE name = $1`
	if lock {
		query += ` FOR UPDATE`
	}
	rows, err := q.Query(ctx, query, name)
	if err != nil {
		return 0, eventstore.Checkpoint{}, false, fmt.Errorf("projection %s: read checkpoint: %w", name, err)
	}
	type row struct {
		Version  int
		TxID     string
		Position int64
	}
	stored, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[row])
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, eventstore.Checkpoint{}, false, nil
	}
	if err != nil {
		return 0, eventstore.Checkpoint{}, false, fmt.Errorf("projection %s: read checkpoint: %w", name, err)
	}
	txid, err := strconv.ParseUint(stored.TxID, 10, 64)
	if err != nil {
		return 0, eventstore.Checkpoint{}, false, fmt.Errorf("projection %s: checkpoint txid: %w", name, err)
	}
	return stored.Version, eventstore.Checkpoint{TxID: txid, Position: stored.Position}, true, nil
}

// observeLag записывает в метрику число событий после позиции проекции. В него входят и события,
// которые ещё нельзя читать из-за незавершённой более ранней транзакции: так долгая транзакция,
// задерживающая всех читателей, видна как растущее отставание.
func (e *Engine) observeLag(ctx context.Context, p Projection) {
	lag, err := e.remaining(ctx, p)
	if err != nil {
		if ctx.Err() == nil {
			e.logger.WarnContext(ctx, "projection lag is unknown", "projection", p.Name, "error", err)
		}
		return
	}
	e.lag.WithLabelValues(p.Name).Set(float64(lag))
}

// remaining возвращает число событий после позиции проекции.
func (e *Engine) remaining(ctx context.Context, p Projection) (int64, error) {
	version, after, _, err := readCheckpoint(ctx, e.pool, p.Name, false)
	if err != nil {
		return 0, err
	}
	if version != p.Version {
		after = eventstore.Checkpoint{} // проекцию ждёт перестроение: впереди все события
	}
	var n int64
	err = e.pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE (txid, position) > ($1::text::xid8, $2)`,
		strconv.FormatUint(after.TxID, 10), after.Position).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("projection %s: count remaining events: %w", p.Name, err)
	}
	return n, nil
}

// sleep ждёт d или отмены ctx; возвращает false, если контекст отменён.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
