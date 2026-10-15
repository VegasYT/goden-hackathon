// Package postgres содержит подключение к базе, транзакции и применение миграций.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"goden/backend/internal/platform/config"
	"goden/backend/migrations"
)

// NewPool создаёт пул и проверяет, что база отвечает.
func NewPool(ctx context.Context, cfg config.Postgres) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		// текст ошибки разбора может содержать строку подключения с паролем
		return nil, fmt.Errorf("postgres: invalid DATABASE_URL")
	}
	poolCfg.MaxConns = cfg.MaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	return pool, nil
}

// InTx выполняет fn в одной транзакции: фиксирует её, если fn вернула nil,
// и откатывает при ошибке или панике.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, fn)
}

// Migrate применяет ещё не применённые миграции и возвращает их имена.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close() //nolint:errcheck // закрывает только обёртку database/sql, пул остаётся открытым

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return nil, fmt.Errorf("postgres: load migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: apply migrations: %w", err)
	}

	applied := make([]string, 0, len(results))
	for _, r := range results {
		applied = append(applied, r.Source.Path)
	}
	return applied, nil
}
