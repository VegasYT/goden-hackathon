// Команда generator эмулирует работу ресторана: создаёт поставки с приёмкой и заказы
// запросами к публичному API (ADR-012). К базе данных процесс не подключается.
// Запуском, остановкой и интенсивностью управляет владелец точки: GET и PUT /api/v1/points/{pointId}/generator.
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/generator"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/platform/observability"
)

// периодичность выполнения накопленных действий
const tick = time.Second

type appConfig struct {
	Log config.Log

	APIURL string `env:"API_URL" envDefault:"http://localhost:8080/api/v1"`
	// Сервисная учётная запись Keycloak. Без ClientID генератор ходит без токена:
	// так можно работать только с API в режиме без авторизации.
	OIDCIssuer   string `env:"OIDC_ISSUER"`
	ClientID     string `env:"GENERATOR_CLIENT_ID"`
	ClientSecret string `env:"GENERATOR_CLIENT_SECRET"`
	// адрес health-эндпоинтов; отличается от API и worker, чтобы процессы запускались на одной машине
	Addr            string        `env:"HTTP_ADDR" envDefault:":8082"`
	Seed            bool          `env:"GENERATOR_SEED" envDefault:"true"`
	PollInterval    time.Duration `env:"GENERATOR_POLL_INTERVAL" envDefault:"2s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generator:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load[appConfig]()
	if err != nil {
		return err
	}
	logger := observability.NewLogger(os.Stdout, cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var tokens oauth2.TokenSource
	switch {
	case cfg.ClientID == "":
		logger.WarnContext(ctx, "generator has no service account: requests carry no access token")
	case cfg.OIDCIssuer == "":
		return errors.New("OIDC_ISSUER is required when GENERATOR_CLIENT_ID is set")
	default:
		if tokens, err = auth.ClientCredentials(ctx, cfg.OIDCIssuer, cfg.ClientID, cfg.ClientSecret); err != nil {
			return err
		}
	}
	// срок на запрос: без него опрос настроек при зависшем API держал бы цикл генератора,
	// и тот не реагировал бы на остановку через настройки
	client, err := oas.NewClient(cfg.APIURL, generator.Security(tokens),
		oas.WithClient(&http.Client{Timeout: 30 * time.Second}))
	if err != nil {
		return fmt.Errorf("api client: %w", err)
	}
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	sim := generator.NewSimulator(client, clock.System{}, rng, logger)
	sim.SeedCatalog = cfg.Seed

	logger.InfoContext(ctx, "generator started", "api_url", cfg.APIURL, "seed", cfg.Seed)
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error { return sim.Run(ctx, cfg.PollInterval, tick) })
	registry := observability.NewRegistry()
	registry.MustRegister(sim.Collectors()...)
	mux := httpx.Health(sim.Ready)
	mux.Handle("GET /metrics", observability.MetricsHandler(registry))
	group.Go(func() error {
		return httpx.Serve(ctx, cfg.Addr, mux, cfg.ShutdownTimeout, logger)
	})
	return group.Wait()
}
