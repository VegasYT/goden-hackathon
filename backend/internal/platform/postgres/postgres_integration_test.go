//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

func setup(t *testing.T) (context.Context, *pgxpool.Pool) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	_, err := pool.Exec(ctx, "CREATE TABLE notes (text text NOT NULL)")
	require.NoError(t, err)
	return ctx, pool
}

func countNotes(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&n))
	return n
}

func TestInTxCommits(t *testing.T) {
	ctx, pool := setup(t)

	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO notes VALUES ('a')")
		return err
	})

	require.NoError(t, err)
	require.Equal(t, 1, countNotes(t, ctx, pool))
}

func TestInTxRollsBackOnError(t *testing.T) {
	ctx, pool := setup(t)
	errBoom := errors.New("boom")

	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO notes VALUES ('a')"); err != nil {
			return err
		}
		return errBoom
	})

	require.ErrorIs(t, err, errBoom)
	require.Equal(t, 0, countNotes(t, ctx, pool))
}

func TestInTxRollsBackOnPanic(t *testing.T) {
	ctx, pool := setup(t)

	require.Panics(t, func() {
		_ = postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "INSERT INTO notes VALUES ('a')")
			panic("boom")
		})
	})

	require.Equal(t, 0, countNotes(t, ctx, pool))
	require.Zero(t, pool.Stat().AcquiredConns(), "connection must be returned to the pool")
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t) // база создана из шаблона с уже применёнными миграциями

	applied, err := postgres.Migrate(ctx, pool)

	require.NoError(t, err)
	require.Empty(t, applied)

	var exists bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass('events') IS NOT NULL").Scan(&exists))
	require.True(t, exists)
}
