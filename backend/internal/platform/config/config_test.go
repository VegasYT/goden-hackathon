package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/config"
)

type appConfig struct {
	Postgres config.Postgres
	Log      config.Log
	Redis    config.Redis
}

const dbURL = "postgres://app:s3cret@localhost:5432/goden"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", dbURL)

	cfg, err := config.Load[appConfig]()

	require.NoError(t, err)
	require.Equal(t, dbURL, cfg.Postgres.URL)
	require.Equal(t, int32(10), cfg.Postgres.MaxConns)
	require.Equal(t, slog.LevelInfo, cfg.Log.Level)
	require.Empty(t, cfg.Redis.URL)
	require.Equal(t, time.Minute, cfg.Redis.CacheTTL)
	require.Equal(t, 100*time.Millisecond, cfg.Redis.Timeout)
}

func TestLoadRedis(t *testing.T) {
	t.Setenv("DATABASE_URL", dbURL)
	t.Setenv("REDIS_URL", "redis://localhost:6379/2")
	t.Setenv("REDIS_CACHE_TTL", "30s")
	t.Setenv("REDIS_TIMEOUT", "75ms")

	cfg, err := config.Load[appConfig]()

	require.NoError(t, err)
	require.Equal(t, "redis://localhost:6379/2", cfg.Redis.URL)
	require.Equal(t, 30*time.Second, cfg.Redis.CacheTTL)
	require.Equal(t, 75*time.Millisecond, cfg.Redis.Timeout)
}

func TestLoadInvalidRedisDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", dbURL)
	t.Setenv("REDIS_CACHE_TTL", "forever")

	_, err := config.Load[appConfig]()

	require.ErrorContains(t, err, "REDIS_CACHE_TTL")
}

func TestLoadMissingRequired(t *testing.T) {
	_, err := config.Load[appConfig]()

	require.ErrorContains(t, err, "DATABASE_URL")
}

func TestLoadInvalidValueIsNotLeaked(t *testing.T) {
	t.Setenv("DATABASE_URL", dbURL)
	t.Setenv("DATABASE_MAX_CONNS", "many-s3cret")

	_, err := config.Load[appConfig]()

	require.ErrorContains(t, err, "DATABASE_MAX_CONNS")
	require.NotContains(t, err.Error(), "s3cret")
}
