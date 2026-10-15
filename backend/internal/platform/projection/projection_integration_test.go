//go:build integration

package projection_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability/metricstest"
	"goden/backend/internal/platform/postgres/pgtest"
	"goden/backend/internal/platform/projection"
)

var (
	ctx       = context.Background()
	startedAt = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
)

type fixture struct {
	pool     *pgxpool.Pool
	clock    *clock.Fake
	registry *prometheus.Registry
	engine   *projection.Engine
}

func setup(t *testing.T) fixture {
	t.Helper()
	pool := pgtest.NewDB(t)
	// Таблицы тестовой проекции: сколько раз применено каждое событие и в каком порядке шли версии потока.
	_, err := pool.Exec(ctx, `
		CREATE TABLE test_applied (event_id uuid PRIMARY KEY, times integer NOT NULL);
		CREATE TABLE test_streams (stream_id text PRIMARY KEY, last_version bigint NOT NULL, out_of_order integer NOT NULL)`)
	require.NoError(t, err)

	clk := clock.NewFake(startedAt)
	registry := prometheus.NewRegistry()
	engine := projection.NewEngine(pool, clk, slog.New(slog.DiscardHandler), registry)
	engine.BatchSize, engine.PollInterval = 5, 10*time.Millisecond
	return fixture{pool, clk, registry, engine}
}

// counting - тестовая проекция. fail, если задана, может отклонить событие.
func counting(name string, version int, fail func(e eventstore.Event) error) projection.Projection {
	return projection.Projection{
		Name: name, Version: version,
		Apply: func(ctx context.Context, tx pgx.Tx, e eventstore.Event) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO test_applied (event_id, times) VALUES ($1, 1)
				ON CONFLICT (event_id) DO UPDATE SET times = test_applied.times + 1`, e.ID)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO test_streams (stream_id, last_version, out_of_order) VALUES ($1, $2::bigint, CASE WHEN $2::bigint = 1 THEN 0 ELSE 1 END)
				ON CONFLICT (stream_id) DO UPDATE SET
					out_of_order = test_streams.out_of_order + CASE WHEN $2::bigint = test_streams.last_version + 1 THEN 0 ELSE 1 END,
					last_version = $2::bigint`, e.StreamID, e.Version)
			if err != nil {
				return err
			}
			if fail != nil {
				return fail(e)
			}
			return nil
		},
		Reset: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `TRUNCATE test_applied, test_streams`)
			return err
		},
	}
}

// write добавляет n событий в поток одной транзакцией и возвращает их идентификаторы.
// Блокировка потока берётся до первой записи, как в командах: иначе порядок чтения
// разошёлся бы с порядком версий. hold удерживает транзакцию открытой перед фиксацией.
func write(pool *pgxpool.Pool, stream string, n int, hold time.Duration, commit bool) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, n)
	events := make([]eventstore.NewEvent, n)
	for i := range events {
		ids[i] = uuid.Must(uuid.NewV7())
		events[i] = eventstore.NewEvent{ID: ids[i], Type: "Noted", SchemaVersion: 1, Payload: []byte(`{}`), OccurredAt: startedAt}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, stream); err != nil {
		return nil, err
	}
	version, err := eventstore.StreamVersion(ctx, tx, stream)
	if err != nil {
		return nil, err
	}
	if _, err := eventstore.Append(ctx, tx, stream, version, events...); err != nil {
		return nil, err
	}
	time.Sleep(hold)
	if !commit {
		return nil, tx.Rollback(ctx)
	}
	return ids, tx.Commit(ctx)
}

func mustWrite(t *testing.T, f fixture, stream string, n int) []uuid.UUID {
	t.Helper()
	ids, err := write(f.pool, stream, n, 0, true)
	require.NoError(t, err)
	return ids
}

type state struct {
	events, applications, outOfOrder int
}

// read возвращает: сколько разных событий применено, сколько всего применений и сколько раз
// версия потока пришла не подряд.
func (f fixture) read(t *testing.T) state {
	t.Helper()
	var s state
	require.NoError(t, f.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM test_applied), (SELECT COALESCE(sum(times), 0) FROM test_applied),
		       (SELECT COALESCE(sum(out_of_order), 0) FROM test_streams)`).Scan(&s.events, &s.applications, &s.outOfOrder))
	return s
}

func (f fixture) snapshot(t *testing.T) string {
	t.Helper()
	var snapshot string
	require.NoError(t, f.pool.QueryRow(ctx, `
		SELECT COALESCE((SELECT json_agg(row_to_json(a) ORDER BY a.event_id) FROM test_applied a)::text, '[]') ||
		       COALESCE((SELECT json_agg(row_to_json(s) ORDER BY s.stream_id) FROM test_streams s)::text, '[]')`).Scan(&snapshot))
	return snapshot
}

func (f fixture) metric(t *testing.T, name, projectionName string) float64 {
	t.Helper()
	return metricstest.Value(t, f.registry, name, "projection", projectionName)
}

// run запускает проектор и останавливает его в конце теста.
func (f fixture) run(t *testing.T, engine *projection.Engine, p projection.Projection) {
	t.Helper()
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- engine.Run(runCtx, p) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err, "Run returns nil when the context is cancelled")
		case <-time.After(10 * time.Second):
			t.Error("Run did not stop")
		}
	})
}

func (f fixture) eventually(t *testing.T, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, 20*time.Second, 10*time.Millisecond)
}

// Применение без пропусков и повторов.

func TestAppliesEveryEventOnceInOrder(t *testing.T) {
	f := setup(t)
	p := counting("stock", 1, nil)
	mustWrite(t, f, "stream-a", 6)
	mustWrite(t, f, "stream-b", 4)
	_, err := write(f.pool, "stream-a", 3, 0, false) // отменённая транзакция
	require.NoError(t, err)
	mustWrite(t, f, "stream-a", 2)

	require.NoError(t, f.engine.CatchUp(ctx, p))
	require.Equal(t, state{events: 12, applications: 12}, f.read(t), "several batches, rolled back events are not applied")

	// повторный вызов и новые события
	require.NoError(t, f.engine.CatchUp(ctx, p))
	mustWrite(t, f, "stream-b", 1)
	require.NoError(t, f.engine.CatchUp(ctx, p))
	require.Equal(t, state{events: 13, applications: 13}, f.read(t))
}

func TestConcurrentWritersWithDelayedCommit(t *testing.T) {
	f := setup(t)
	f.run(t, f.engine, counting("stock", 1, nil))

	const writers, perWriter = 8, 25
	var written atomic.Int64
	var g errgroup.Group
	for w := range writers {
		g.Go(func() error {
			rng := rand.New(rand.NewPCG(uint64(w), 7))
			for i := range perWriter {
				// свой поток и общий: за общий писатели конкурируют
				stream := fmt.Sprintf("own-%d", w)
				if i%3 == 0 {
					stream = fmt.Sprintf("shared-%d", rng.IntN(2))
				}
				n := 1 + rng.IntN(3)
				// транзакция, начатая раньше, часто фиксируется позже соседней
				ids, err := write(f.pool, stream, n, time.Duration(rng.IntN(50))*time.Millisecond, rng.IntN(10) != 0)
				if err != nil {
					return err
				}
				written.Add(int64(len(ids)))
			}
			return nil
		})
	}
	require.NoError(t, g.Wait())

	total := int(written.Load())
	require.Greater(t, total, writers*perWriter/2)
	f.eventually(t, func() bool { return f.read(t).events == total })
	require.Equal(t, state{events: total, applications: total}, f.read(t),
		"every event exactly once, versions of every stream in order")
	// датчик обновляется после порции, чуть позже самих данных
	f.eventually(t, func() bool { return f.metric(t, "goden_projection_lag_events", "stock") == 0 })
}

// Checkpoint в одной транзакции с данными.

func TestReadCheckpoint(t *testing.T) {
	f := setup(t)

	_, found, err := projection.ReadCheckpoint(ctx, f.pool, "stock")
	require.NoError(t, err)
	require.False(t, found)

	mustWrite(t, f, "stream-a", 1)
	require.NoError(t, f.engine.CatchUp(ctx, counting("stock", 3, nil)))
	first, found, err := projection.ReadCheckpoint(ctx, f.pool, "stock")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 3, first.Version)
	require.NotZero(t, first.TxID)
	require.NotZero(t, first.Position)

	mustWrite(t, f, "stream-a", 1)
	require.NoError(t, f.engine.CatchUp(ctx, counting("stock", 3, nil)))
	second, found, err := projection.ReadCheckpoint(ctx, f.pool, "stock")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, second.Position, first.Position)
}

func TestFailedBatchChangesNothing(t *testing.T) {
	f := setup(t)
	ids := mustWrite(t, f, "stream-a", 5)
	broken := true
	p := counting("stock", 1, func(e eventstore.Event) error {
		if broken && e.ID == ids[2] {
			return errors.New("cannot apply")
		}
		return nil
	})

	err := f.engine.CatchUp(ctx, p)
	require.ErrorContains(t, err, "cannot apply")
	require.Equal(t, state{}, f.read(t), "data of the two applied events is rolled back with the position")

	broken = false
	require.NoError(t, f.engine.CatchUp(ctx, p))
	require.Equal(t, state{events: 5, applications: 5}, f.read(t))
}

func TestRestartContinuesFromCheckpoint(t *testing.T) {
	f := setup(t)
	mustWrite(t, f, "stream-a", 7)
	require.NoError(t, f.engine.CatchUp(ctx, counting("stock", 1, nil)))

	// новый экземпляр движка, как после перезапуска процесса
	restarted := projection.NewEngine(f.pool, f.clock, slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	mustWrite(t, f, "stream-a", 3)
	require.NoError(t, restarted.CatchUp(ctx, counting("stock", 1, nil)))
	require.Equal(t, state{events: 10, applications: 10}, f.read(t))
}

func TestTwoProjectorsOfOneProjection(t *testing.T) {
	f := setup(t)
	second := projection.NewEngine(f.pool, f.clock, slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	second.BatchSize, second.PollInterval = 3, 7*time.Millisecond
	f.run(t, f.engine, counting("stock", 1, nil))
	f.run(t, second, counting("stock", 1, nil))

	total := 0
	for i := range 60 {
		total += len(mustWrite(t, f, fmt.Sprintf("stream-%d", i%4), 1+i%3))
	}
	f.eventually(t, func() bool { return f.read(t).events == total })
	time.Sleep(100 * time.Millisecond) // второй проектор успел бы применить повторно
	require.Equal(t, state{events: total, applications: total}, f.read(t))
}

// Независимость проекций.

func TestProjectionsAreIndependent(t *testing.T) {
	f := setup(t)
	_, err := f.pool.Exec(ctx, `CREATE TABLE test_other (event_id uuid PRIMARY KEY)`)
	require.NoError(t, err)
	other := projection.Projection{
		Name: "other", Version: 1,
		Apply: func(ctx context.Context, tx pgx.Tx, e eventstore.Event) error {
			_, err := tx.Exec(ctx, `INSERT INTO test_other (event_id) VALUES ($1)`, e.ID)
			return err
		},
		Reset: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `TRUNCATE test_other`)
			return err
		},
	}
	countOther := func() (n int) {
		require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM test_other`).Scan(&n))
		return n
	}

	mustWrite(t, f, "stream-a", 4)
	f.run(t, f.engine, counting("stock", 1, func(eventstore.Event) error { return errors.New("always broken") }))
	f.run(t, f.engine, other)

	f.eventually(t, func() bool { return countOther() == 4 })
	mustWrite(t, f, "stream-a", 2)
	f.eventually(t, func() bool { return countOther() == 6 })
	require.Equal(t, state{}, f.read(t), "the broken projection does not advance")
	f.eventually(t, func() bool { return f.metric(t, "goden_projection_errors_total", "stock") >= 2 })
	require.Zero(t, f.metric(t, "goden_projection_errors_total", "other"))
	// отставание пересчитывается при каждой неудачной попытке, а пауза между ними растёт
	f.eventually(t, func() bool { return f.metric(t, "goden_projection_lag_events", "stock") == 6 })
}

// Перестроение.

func TestRebuildOnVersionChange(t *testing.T) {
	f := setup(t)
	mustWrite(t, f, "stream-a", 9)
	mustWrite(t, f, "stream-b", 8)
	require.NoError(t, f.engine.CatchUp(ctx, counting("stock", 1, nil)))
	original := f.snapshot(t)

	// та же версия: ничего не очищается
	resets := 0
	same := counting("stock", 1, nil)
	reset := same.Reset
	same.Reset = func(ctx context.Context, tx pgx.Tx) error { resets++; return reset(ctx, tx) }
	require.NoError(t, f.engine.CatchUp(ctx, same))
	require.Zero(t, resets)
	require.Equal(t, state{events: 17, applications: 17}, f.read(t))

	// новая версия: данные очищены один раз, все события применены заново несколькими порциями
	next := counting("stock", 2, nil)
	next.Reset = same.Reset
	batches := 0
	apply := next.Apply
	var lastTx pgx.Tx
	next.Apply = func(ctx context.Context, tx pgx.Tx, e eventstore.Event) error {
		if tx != lastTx {
			batches, lastTx = batches+1, tx
		}
		return apply(ctx, tx, e)
	}
	require.NoError(t, f.engine.CatchUp(ctx, next))
	require.Equal(t, 1, resets)
	require.Greater(t, batches, 1, "the rebuild is not one long transaction")
	require.Equal(t, state{events: 17, applications: 17}, f.read(t), "old data is gone, nothing is applied twice")
	require.Equal(t, original, f.snapshot(t), "the rebuilt projection equals the original")

	// версия сохранена: следующий запуск не перестраивает
	require.NoError(t, f.engine.CatchUp(ctx, next))
	require.Equal(t, 1, resets)
}

func TestNewProjectionStartsFromTheBeginning(t *testing.T) {
	f := setup(t)
	mustWrite(t, f, "stream-a", 6)
	f.run(t, f.engine, counting("late", 1, nil))
	f.eventually(t, func() bool { return f.read(t).events == 6 })
}

// Устойчивость и метрики.

func TestRunRetriesAfterAnError(t *testing.T) {
	f := setup(t)
	mustWrite(t, f, "stream-a", 3)
	var failures atomic.Int32
	f.run(t, f.engine, counting("stock", 1, func(eventstore.Event) error {
		if failures.Add(1) <= 2 {
			return errors.New("temporary")
		}
		return nil
	}))

	f.eventually(t, func() bool { return f.read(t).events == 3 })
	mustWrite(t, f, "stream-a", 2)
	f.eventually(t, func() bool { return f.read(t).events == 5 })
	require.Equal(t, state{events: 5, applications: 5}, f.read(t))
	require.EqualValues(t, 2, f.metric(t, "goden_projection_errors_total", "stock"))
}

func TestProjectionMetrics(t *testing.T) {
	f := setup(t)
	p := counting("stock", 1, nil)
	mustWrite(t, f, "stream-a", 4)
	require.NoError(t, f.engine.CatchUp(ctx, p))
	require.EqualValues(t, 4, f.metric(t, "goden_projection_events_total", "stock"))
	require.Zero(t, f.metric(t, "goden_projection_lag_events", "stock"))
	require.EqualValues(t, startedAt.Unix(), f.metric(t, "goden_projection_last_success_timestamp_seconds", "stock"))

	// новые события видны как отставание до того, как проектор их применит
	mustWrite(t, f, "stream-a", 3)
	broken := counting("stock", 1, func(eventstore.Event) error { return errors.New("cannot apply") })
	f.clock.Advance(time.Minute)
	f.run(t, f.engine, broken)
	f.eventually(t, func() bool { return f.metric(t, "goden_projection_errors_total", "stock") >= 1 })
	f.eventually(t, func() bool { return f.metric(t, "goden_projection_lag_events", "stock") == 3 })
	require.EqualValues(t, 4, f.metric(t, "goden_projection_events_total", "stock"))
	require.EqualValues(t, startedAt.Unix(), f.metric(t, "goden_projection_last_success_timestamp_seconds", "stock"),
		"a failing projection has no new success")
}

func TestInvalidProjectionIsRejected(t *testing.T) {
	f := setup(t)
	require.Error(t, f.engine.CatchUp(ctx, projection.Projection{Name: "", Version: 1}))
	require.Error(t, f.engine.Run(ctx, projection.Projection{Name: "stock", Version: 1}))
}
