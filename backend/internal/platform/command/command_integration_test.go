//go:build integration

package command_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres/pgtest"
)

type result struct {
	Version int64 `json:"version"`
}

type fixture struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	clock  *clock.Fake
	runner *command.Runner
}

func setup(t *testing.T) fixture {
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(startedAt)
	return fixture{context.Background(), pool, clk, command.NewRunner(pool, clk, quiet)}
}

func spec(name string, locks ...string) command.Spec {
	return command.Spec{Name: name, Actor: manager, Locks: locks}
}

// appendNote читает версию потока и добавляет в него событие, как это делает обычная команда.
func appendNote(ctx context.Context, tx *command.Tx, stream string) (result, error) {
	version, err := eventstore.StreamVersion(ctx, tx, stream)
	if err != nil {
		return result{}, err
	}
	version, err = tx.Append(ctx, stream, version, noted{Note: stream})
	return result{Version: version}, err
}

func (f fixture) version(t *testing.T, stream string) int64 {
	t.Helper()
	v, err := eventstore.StreamVersion(f.ctx, f.pool, stream)
	require.NoError(t, err)
	return v
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

// Атомарность.

func TestRunCommitsEventsAndTables(t *testing.T) {
	f := setup(t)
	_, err := f.pool.Exec(f.ctx, "CREATE TABLE balances (stream text PRIMARY KEY)")
	require.NoError(t, err)

	_, err = command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
		if _, err := appendNote(ctx, tx, "a"); err != nil {
			return result{}, err
		}
		if _, err := appendNote(ctx, tx, "b"); err != nil {
			return result{}, err
		}
		_, err := tx.Exec(ctx, "INSERT INTO balances VALUES ('a')")
		return result{}, err
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), f.version(t, "a"))
	require.Equal(t, int64(1), f.version(t, "b"))
	var rows int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM balances").Scan(&rows))
	require.Equal(t, 1, rows)
}

func TestRunRollsBackOnError(t *testing.T) {
	f := setup(t)
	errRule := errors.New("business rule violated")

	_, err := command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
		if _, err := appendNote(ctx, tx, "a"); err != nil {
			return result{}, err
		}
		return result{}, errRule
	})

	require.ErrorIs(t, err, errRule)
	require.Zero(t, f.version(t, "a"))
}

func TestRunRollsBackOnPanic(t *testing.T) {
	f := setup(t)

	require.Panics(t, func() {
		_, _ = command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
			_, _ = appendNote(ctx, tx, "a")
			panic("boom")
		})
	})

	require.Zero(t, f.version(t, "a"))
	require.Zero(t, f.pool.Stat().AcquiredConns())
}

// Служебные поля событий.

func TestEventEnvelope(t *testing.T) {
	f := setup(t)
	correlationID := uuid.Must(uuid.NewV7())
	ctx := observability.WithCorrelationID(f.ctx, correlationID)

	_, err := command.Run(ctx, f.runner, spec("RegisterDelivery"), func(ctx context.Context, tx *command.Tx) (result, error) {
		require.Equal(t, startedAt, tx.Now())
		if _, err := tx.Append(ctx, "a", 0, noted{Note: "1"}, noted{Note: "2"}); err != nil {
			return result{}, err
		}
		_, err := tx.Append(ctx, "b", 0, noted{Note: "3"})
		return result{}, err
	})
	require.NoError(t, err)

	events, err := eventstore.ReadAll(f.ctx, f.pool, eventstore.Checkpoint{}, 10)
	require.NoError(t, err)
	require.Len(t, events, 3)

	seen := map[uuid.UUID]bool{}
	for _, e := range events {
		seen[e.ID] = true
		require.Equal(t, correlationID, e.CorrelationID)
		require.Equal(t, events[0].CausationID, e.CausationID)
		require.NotEqual(t, uuid.Nil, e.CausationID)
		require.Equal(t, startedAt, e.OccurredAt)
		require.Equal(t, "Noted", e.Type)
		require.JSONEq(t, `{"actor":{"type":"user","id":"user-1"},"command":"RegisterDelivery"}`, string(e.Metadata))
	}
	require.Len(t, seen, 3, "event ids must differ")

	decoded, err := eventstore.Decode(events[2].Type, events[2].SchemaVersion, events[2].Payload)
	require.NoError(t, err)
	require.Equal(t, noted{Note: "3"}, decoded)
}

func TestCorrelationIDIsCreatedWhenContextHasNone(t *testing.T) {
	f := setup(t)

	_, err := command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
		_, err := tx.Append(ctx, "a", 0, noted{}, noted{})
		return result{}, err
	})
	require.NoError(t, err)

	events, err := eventstore.ReadStream(f.ctx, f.pool, "a", 1)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, events[0].CorrelationID)
	require.Equal(t, events[0].CorrelationID, events[1].CorrelationID)
}

// Блокировки.

func TestCommandsOnSameStreamRunInTurn(t *testing.T) {
	f := setup(t)
	firstEntered, release := make(chan struct{}), make(chan struct{})
	var calls, secondEntered atomic.Int32

	var g errgroup.Group
	g.Go(func() error {
		_, err := command.Run(f.ctx, f.runner, spec("First", "stock-1"), func(ctx context.Context, tx *command.Tx) (result, error) {
			calls.Add(1)
			require.True(t, tx.Locked("stock-1"))
			require.False(t, tx.Locked("stock-2"))
			close(firstEntered)
			<-release
			return appendNote(ctx, tx, "stock-1")
		})
		return err
	})
	waitFor(t, firstEntered, "first command")

	g.Go(func() error {
		_, err := command.Run(f.ctx, f.runner, spec("Second", "stock-1"), func(ctx context.Context, tx *command.Tx) (result, error) {
			calls.Add(1)
			secondEntered.Add(1)
			return appendNote(ctx, tx, "stock-1")
		})
		return err
	})

	// вторая команда не начинает логику, пока первая держит поток
	time.Sleep(300 * time.Millisecond)
	require.Zero(t, secondEntered.Load())

	close(release)
	require.NoError(t, g.Wait())
	require.Equal(t, int32(2), calls.Load(), "no retries: each command ran once")
	require.Equal(t, int64(2), f.version(t, "stock-1"))
}

func TestCommandsOnDifferentStreamsRunInParallel(t *testing.T) {
	f := setup(t)
	firstEntered, release := make(chan struct{}), make(chan struct{})

	var g errgroup.Group
	g.Go(func() error {
		_, err := command.Run(f.ctx, f.runner, spec("First", "stock-a"), func(ctx context.Context, tx *command.Tx) (result, error) {
			close(firstEntered)
			<-release
			return appendNote(ctx, tx, "stock-a")
		})
		return err
	})
	waitFor(t, firstEntered, "first command")

	secondDone := make(chan struct{})
	g.Go(func() error {
		defer close(secondDone)
		_, err := command.Run(f.ctx, f.runner, spec("Second", "stock-b"), func(ctx context.Context, tx *command.Tx) (result, error) {
			return appendNote(ctx, tx, "stock-b")
		})
		return err
	})

	waitFor(t, secondDone, "second command while the first one is still running")
	close(release)
	require.NoError(t, g.Wait())
}

func TestLockOrderDoesNotCauseDeadlocks(t *testing.T) {
	f := setup(t)
	const pairs = 50
	var calls atomic.Int32

	both := func(ctx context.Context, tx *command.Tx) (result, error) {
		calls.Add(1)
		if _, err := appendNote(ctx, tx, "stock-a"); err != nil {
			return result{}, err
		}
		return appendNote(ctx, tx, "stock-b")
	}

	var g errgroup.Group
	for range pairs {
		g.Go(func() error {
			_, err := command.Run(f.ctx, f.runner, spec("AB", "stock-a", "stock-b"), both)
			return err
		})
		g.Go(func() error {
			_, err := command.Run(f.ctx, f.runner, spec("BA", "stock-b", "stock-a"), both)
			return err
		})
	}

	require.NoError(t, g.Wait())
	require.Equal(t, int32(2*pairs), calls.Load(), "no deadlocks and no retries")
	require.Equal(t, int64(2*pairs), f.version(t, "stock-a"))
	require.Equal(t, int64(2*pairs), f.version(t, "stock-b"))
}

// Читатели получают события в порядке (txid, position). Для потока под блокировкой
// он обязан совпадать с порядком версий, иначе проекции применят события не по порядку.
func TestGlobalOrderMatchesStreamOrder(t *testing.T) {
	f := setup(t)
	const workers, perWorker = 8, 25

	var g errgroup.Group
	for range workers {
		g.Go(func() error {
			for range perWorker {
				_, err := command.Run(f.ctx, f.runner, spec("Consume", "stock-1"),
					func(ctx context.Context, tx *command.Tx) (result, error) { return appendNote(ctx, tx, "stock-1") })
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	require.NoError(t, g.Wait())

	events, err := eventstore.ReadAll(f.ctx, f.pool, eventstore.Checkpoint{}, 1000)
	require.NoError(t, err)
	require.Len(t, events, workers*perWorker)
	for i, e := range events {
		require.Equal(t, int64(i+1), e.Version, "event at global index %d", i)
	}
}

// Идемпотентность.

func TestIdempotentCommandRunsOnce(t *testing.T) {
	f := setup(t)
	var calls atomic.Int32
	s := spec("AcceptOrder")
	s.IdempotencyKey, s.Request = "key-1", map[string]any{"dish": "pasta", "portions": 2}
	run := func() (result, error) {
		return command.Run(f.ctx, f.runner, s, func(ctx context.Context, tx *command.Tx) (result, error) {
			calls.Add(1)
			return appendNote(ctx, tx, "order-1")
		})
	}

	first, err := run()
	require.NoError(t, err)
	second, err := run()
	require.NoError(t, err)

	require.Equal(t, result{Version: 1}, first)
	require.Equal(t, first, second)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, int64(1), f.version(t, "order-1"))
}

func TestIdempotencyKeyReusedWithDifferentRequest(t *testing.T) {
	f := setup(t)
	s := spec("AcceptOrder")
	s.IdempotencyKey, s.Request = "key-1", map[string]any{"portions": 2}
	note := func(ctx context.Context, tx *command.Tx) (result, error) { return appendNote(ctx, tx, "order-1") }
	_, err := command.Run(f.ctx, f.runner, s, note)
	require.NoError(t, err)

	s.Request = map[string]any{"portions": 3}
	_, err = command.Run(f.ctx, f.runner, s, note)

	require.ErrorIs(t, err, command.ErrKeyReused)
	require.Equal(t, int64(1), f.version(t, "order-1"))
}

func TestSameKeyInDifferentCommandsIsIndependent(t *testing.T) {
	f := setup(t)
	note := func(ctx context.Context, tx *command.Tx) (result, error) { return appendNote(ctx, tx, "s") }

	for _, name := range []string{"AcceptOrder", "RegisterDelivery"} {
		s := spec(name)
		s.IdempotencyKey = "shared-key"
		_, err := command.Run(f.ctx, f.runner, s, note)
		require.NoError(t, err)
	}

	require.Equal(t, int64(2), f.version(t, "s"))
}

func TestCommandWithoutKeyRunsEveryTime(t *testing.T) {
	f := setup(t)
	note := func(ctx context.Context, tx *command.Tx) (result, error) { return appendNote(ctx, tx, "s") }

	for range 2 {
		_, err := command.Run(f.ctx, f.runner, spec("Test"), note)
		require.NoError(t, err)
	}

	require.Equal(t, int64(2), f.version(t, "s"))
}

func TestConcurrentCallsWithSameKeyRunOnce(t *testing.T) {
	f := setup(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s := spec("AcceptOrder")
	s.IdempotencyKey = "key-1"
	run := func() (result, error) {
		return command.Run(f.ctx, f.runner, s, func(ctx context.Context, tx *command.Tx) (result, error) {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
			}
			return appendNote(ctx, tx, "order-1")
		})
	}

	results := make([]result, 2)
	var g errgroup.Group
	g.Go(func() (err error) { results[0], err = run(); return err })
	waitFor(t, entered, "first call")
	g.Go(func() (err error) { results[1], err = run(); return err }) // ждёт на занятом ключе
	time.Sleep(300 * time.Millisecond)
	close(release)

	require.NoError(t, g.Wait())
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, results[0], results[1])
	require.Equal(t, int64(1), f.version(t, "order-1"))
}

func TestFailedCommandDoesNotHoldKey(t *testing.T) {
	f := setup(t)
	var calls atomic.Int32
	errRule := errors.New("not enough stock")
	s := spec("AcceptOrder")
	s.IdempotencyKey = "key-1"
	run := func() (result, error) {
		return command.Run(f.ctx, f.runner, s, func(ctx context.Context, tx *command.Tx) (result, error) {
			if calls.Add(1) == 1 {
				return result{}, errRule
			}
			return appendNote(ctx, tx, "order-1")
		})
	}

	_, err := run()
	require.ErrorIs(t, err, errRule)
	got, err := run()

	require.NoError(t, err)
	require.Equal(t, result{Version: 1}, got)
	require.Equal(t, int32(2), calls.Load())
}

// Повтор при конфликте.

func TestRetryOnVersionConflict(t *testing.T) {
	f := setup(t)
	var calls atomic.Int32

	got, err := command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
		res, err := appendNote(ctx, tx, "s")
		if err != nil {
			return result{}, err
		}
		if calls.Add(1) == 1 {
			return result{}, fmt.Errorf("state changed: %w", eventstore.ErrVersionConflict)
		}
		return res, nil
	})

	require.NoError(t, err)
	require.Equal(t, result{Version: 1}, got)
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, int64(1), f.version(t, "s"), "the failed attempt left no events")
}

func TestGivesUpAfterRepeatedConflicts(t *testing.T) {
	f := setup(t)
	var calls atomic.Int32

	_, err := command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
		calls.Add(1)
		if _, err := appendNote(ctx, tx, "s"); err != nil {
			return result{}, err
		}
		return result{}, eventstore.ErrVersionConflict
	})

	require.ErrorIs(t, err, eventstore.ErrVersionConflict)
	require.Equal(t, int32(3), calls.Load())
	require.Zero(t, f.version(t, "s"))
}

func TestBusinessErrorIsNotRetried(t *testing.T) {
	f := setup(t)
	var calls atomic.Int32
	errRule := errors.New("delivery already approved")

	_, err := command.Run(f.ctx, f.runner, spec("Test"), func(context.Context, *command.Tx) (result, error) {
		calls.Add(1)
		return result{}, errRule
	})

	require.ErrorIs(t, err, errRule)
	require.Equal(t, int32(1), calls.Load())
}

// Две команды без блокировок читают одну версию потока и пишут в него:
// одна получает настоящий конфликт от Event Store и проходит со второй попытки.
func TestRealConflictWithoutLocksIsRetried(t *testing.T) {
	f := setup(t)
	var calls atomic.Int32
	var bothRead sync.WaitGroup
	bothRead.Add(2)

	run := func() error {
		first := true
		_, err := command.Run(f.ctx, f.runner, spec("Test"), func(ctx context.Context, tx *command.Tx) (result, error) {
			calls.Add(1)
			version, err := eventstore.StreamVersion(ctx, tx, "delivery-1")
			if err != nil {
				return result{}, err
			}
			if first {
				first = false
				bothRead.Done()
				bothRead.Wait() // обе команды прочитали версию 0
			}
			version, err = tx.Append(ctx, "delivery-1", version, noted{})
			return result{Version: version}, err
		})
		return err
	}

	var g errgroup.Group
	g.Go(run)
	g.Go(run)

	require.NoError(t, g.Wait())
	require.Equal(t, int64(2), f.version(t, "delivery-1"))
	require.Equal(t, int32(3), calls.Load(), "one command retried once")
}

// Ключ принадлежит автору: тот же ключ и то же тело от другого автора - независимая команда.
// Второй автор не получает чужой результат и не получает отказ, по которому узнал бы о чужом ключе.
func TestIdempotencyKeyBelongsToTheActor(t *testing.T) {
	f := setup(t)
	s := spec("AcceptOrder")
	s.IdempotencyKey, s.Request = "key-1", map[string]any{"portions": 2}
	note := func(ctx context.Context, tx *command.Tx) (result, error) { return appendNote(ctx, tx, "order-1") }
	_, err := command.Run(f.ctx, f.runner, s, note)
	require.NoError(t, err)

	first := s.Actor
	s.Actor = eventstore.Actor{Type: eventstore.ActorUser, ID: "another-user"}
	_, err = command.Run(f.ctx, f.runner, s, note)
	require.NoError(t, err)
	require.Equal(t, int64(2), f.version(t, "order-1"), "the command of another actor is executed")

	// повтор каждого автора возвращает его результат и ничего не пишет
	for _, actor := range []eventstore.Actor{first, s.Actor} {
		s.Actor = actor
		_, err = command.Run(f.ctx, f.runner, s, note)
		require.NoError(t, err)
	}
	require.Equal(t, int64(2), f.version(t, "order-1"))
}
