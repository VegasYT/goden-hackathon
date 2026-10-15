// Команда api запускает HTTP API. Миграции она не применяет: это делает cmd/migrate.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"goden/backend/internal/api"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/httpx"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres"
	platformredis "goden/backend/internal/platform/redis"
	"goden/backend/internal/realtime"
)

type appConfig struct {
	Postgres config.Postgres
	Log      config.Log
	Kafka    platformkafka.Config
	Redis    config.Redis

	Addr string `env:"HTTP_ADDR" envDefault:":8080"`
	// AuthDisabled включает режим без авторизации: только для локальной разработки
	AuthDisabled bool `env:"AUTH_DISABLED" envDefault:"false"`
	// OIDCIssuer - адрес realm Keycloak, например http://localhost:8180/realms/goden
	OIDCIssuer   string   `env:"OIDC_ISSUER"`
	OIDCAudience string   `env:"OIDC_AUDIENCE" envDefault:"goden-api"`
	CORSOrigins  []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"http://localhost:5173"`

	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

// trimmed убирает пробелы вокруг значений списка: "http://a, http://b" иначе даёт адрес
// с пробелом в начале, который не совпадает ни с одним источником.
func trimmed(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func run() error {
	cfg, err := config.Load[appConfig]()
	if err != nil {
		return err
	}
	logger := observability.NewLogger(os.Stdout, cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	readCache, err := platformredis.New(cfg.Redis, logger, pool)
	if err != nil {
		return err
	}
	defer readCache.Close() //nolint:errcheck

	hub := realtime.NewHub()
	reader, err := realtime.NewReader(cfg.Kafka, hub, logger)
	if err != nil {
		return err
	}
	defer reader.Close()

	opts := api.Options{
		Verifier: auth.Disabled(), AllowAnonymous: cfg.AuthDisabled,
		CORSOrigins: trimmed(cfg.CORSOrigins), Realtime: hub, ReadCache: readCache,
	}
	switch {
	case cfg.AuthDisabled:
		logger.WarnContext(ctx, "authorization is disabled: every request runs as the dev user, do not expose this server")
	case cfg.OIDCIssuer == "":
		return errors.New("OIDC_ISSUER is required; for local development without Keycloak set AUTH_DISABLED=true")
	default:
		if opts.Verifier, err = auth.NewOIDC(ctx, cfg.OIDCIssuer, cfg.OIDCAudience, clock.System{}); err != nil {
			return err
		}
	}

	handler, err := api.New(pool, clock.System{}, logger, opts)
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error { return httpx.Serve(ctx, cfg.Addr, handler, cfg.ShutdownTimeout, logger) })
	group.Go(func() error { return reader.Run(ctx) })
	// при остановке потоки SSE закрываются сразу: сервер ждёт, пока соединения закроются сами,
	// а открытый поток держал бы его до истечения срока остановки
	group.Go(func() error {
		<-ctx.Done()
		hub.Close()
		return nil
	})
	return group.Wait()
}
