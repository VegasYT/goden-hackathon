//go:build integration

package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability/metricstest"
)

func TestCommandMetrics(t *testing.T) {
	f := setup(t)
	registry := f.runner.Registry()
	commands := func(name, outcome string) float64 {
		return metricstest.Value(t, registry, "goden_commands_total", "command", name, "outcome", outcome)
	}
	retries := func(name string) float64 {
		return metricstest.Value(t, registry, "goden_command_retries_total", "command", name)
	}
	events := func() float64 {
		return metricstest.Value(t, registry, "goden_events_appended_total", "type", noted{}.EventType())
	}
	note := func(ctx context.Context, tx *command.Tx) (result, error) { return appendNote(ctx, tx, "stock-1") }

	// успех
	_, err := command.Run(f.ctx, f.runner, spec("Plain", "stock-1"), note)
	require.NoError(t, err)
	require.EqualValues(t, 1, commands("Plain", "ok"))
	require.EqualValues(t, 1, events())

	// повтор по ключу идемпотентности: команда не выполняется, событий нет
	keyed := spec("Keyed", "stock-1")
	keyed.IdempotencyKey, keyed.Request = "key-1", "request"
	for range 3 {
		_, err = command.Run(f.ctx, f.runner, keyed, note)
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, commands("Keyed", "ok"))
	require.EqualValues(t, 2, commands("Keyed", "replayed"))
	require.EqualValues(t, 2, events(), "a replayed command appends nothing")

	// конфликт версий: два повтора, затем успех; события откатившихся попыток не считаются
	attempts := 0
	_, err = command.Run(f.ctx, f.runner, spec("Contended", "stock-1"), func(ctx context.Context, tx *command.Tx) (result, error) {
		attempts++
		res, err := appendNote(ctx, tx, "stock-1")
		if err != nil || attempts < 3 {
			return result{}, errors.Join(err, eventstore.ErrVersionConflict)
		}
		return res, nil
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, retries("Contended"))
	require.EqualValues(t, 1, commands("Contended", "ok"))
	require.EqualValues(t, 3, events(), "only the committed attempt counts")

	// повторы исчерпаны
	_, err = command.Run(f.ctx, f.runner, spec("Hopeless", "stock-1"), func(context.Context, *command.Tx) (result, error) {
		return result{}, eventstore.ErrVersionConflict
	})
	require.ErrorIs(t, err, eventstore.ErrVersionConflict)
	require.EqualValues(t, 1, commands("Hopeless", "exhausted"))
	require.Zero(t, commands("Hopeless", "ok"))

	// ошибка предметной области: без повторов
	_, err = command.Run(f.ctx, f.runner, spec("Rejected", "stock-1"), func(ctx context.Context, tx *command.Tx) (result, error) {
		if _, err := appendNote(ctx, tx, "stock-1"); err != nil {
			return result{}, err
		}
		return result{}, errors.New("rule violated")
	})
	require.Error(t, err)
	require.EqualValues(t, 1, commands("Rejected", "error"))
	require.Zero(t, retries("Rejected"))
	require.EqualValues(t, 3, events(), "a failed command appends nothing")
}

func TestOnCommitRunsOnlyAfterCommit(t *testing.T) {
	f := setup(t)
	var committed int
	run := func(fail bool) error {
		_, err := command.Run(f.ctx, f.runner, spec("Hooked", "stock-1"), func(_ context.Context, tx *command.Tx) (result, error) {
			tx.OnCommit(func() { committed++ })
			if fail {
				return result{}, errors.New("rule violated")
			}
			return result{}, nil
		})
		return err
	}
	require.Error(t, run(true))
	require.Zero(t, committed)
	require.NoError(t, run(false))
	require.Equal(t, 1, committed)
}
