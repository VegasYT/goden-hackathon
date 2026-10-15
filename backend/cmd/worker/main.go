// Команда worker запускает фоновые задачи: проекторы, публикацию событий, контроль сроков
// годности, уведомления. Сейчас он выполняет проекции: без него списки поставок и заказов
// не обновляются. Новые задачи добавляются в функцию tasks.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/notifications"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/httpx"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/projection"
	"goden/backend/internal/readmodel"
)

type appConfig struct {
	Postgres config.Postgres
	Log      config.Log
	Kafka    platformkafka.Config
	Telegram telegramConfig

	// адрес health-эндпоинтов; отличается от API, чтобы оба процесса запускались на одной машине
	Addr string `env:"HTTP_ADDR" envDefault:":8081"`
	// ExpiryWarningPeriod - за сколько до окончания срока годности выдаётся предупреждение по партии
	ExpiryWarningPeriod time.Duration `env:"EXPIRY_WARNING_PERIOD" envDefault:"24h"`
	// ExpiryCheckInterval - как часто проверяются сроки годности
	ExpiryCheckInterval time.Duration `env:"EXPIRY_CHECK_INTERVAL" envDefault:"1m"`

	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

type telegramConfig struct {
	Token  string `env:"TELEGRAM_BOT_TOKEN"`
	ChatID string `env:"TELEGRAM_CHAT_ID"`
}

func (c telegramConfig) resolve() (int64, bool, error) {
	switch {
	case c.Token == "" && c.ChatID == "":
		return 0, false, nil
	case c.Token == "":
		return 0, false, fmt.Errorf("config: TELEGRAM_BOT_TOKEN is required when TELEGRAM_CHAT_ID is set")
	case c.ChatID == "":
		return 0, false, fmt.Errorf("config: TELEGRAM_CHAT_ID is required when TELEGRAM_BOT_TOKEN is set")
	}
	chatID, err := strconv.ParseInt(c.ChatID, 10, 64)
	if err != nil || chatID == 0 {
		return 0, false, fmt.Errorf("config: TELEGRAM_CHAT_ID has invalid value, expected integer")
	}
	return chatID, true, nil
}

// task - фоновая задача. Она работает до отмены контекста и возвращает nil при штатной остановке.
type task func(ctx context.Context) error

// tasks возвращает фоновые задачи процесса и реестр их метрик. Новая задача добавляется сюда.
func tasks(pool *pgxpool.Pool, clk clock.Clock, logger *slog.Logger, cfg appConfig,
	producer platformkafka.Producer, notificationSender notifications.Sender,
) ([]task, *prometheus.Registry, func(), error) {
	// реестр метрик процесса - у исполнителя команд: в нём же счётчики команд, которые вызывает worker
	runner := command.NewRunner(pool, clk, logger)
	registry := runner.Registry()

	// каждая проекция - отдельная задача: у неё своя позиция, и сбой одной не останавливает остальные
	engine := projection.NewEngine(pool, clk, logger, registry)
	var out []task
	for _, p := range readmodel.Projections() {
		out = append(out, func(ctx context.Context) error { return engine.Run(ctx, p) })
	}

	// контроль сроков годности: предупреждения и списание просрочки готовыми командами
	stock := inventory.NewService(runner, pool, catalog.NewService(runner, pool))
	expiry := inventory.NewExpiryChecker(stock, logger, cfg.ExpiryWarningPeriod, cfg.ExpiryCheckInterval)
	out = append(out, expiry.Run)

	// publisher - единственная точка записи доменных событий в Kafka. В тестах worker
	// producer можно не передавать, чтобы проверять остальные фоновые задачи без broker.
	if producer != nil {
		publisher := platformkafka.NewPublisher(pool, producer, clk, logger, registry, cfg.Kafka.TopicPrefix)
		out = append(out, publisher.Run)
	}

	closeNotifications := func() {}
	if notificationSender != nil {
		queue := notifications.NewQueue(pool)
		domainConsumer, err := platformkafka.NewConsumer(pool, cfg.Kafka, "notifications-domain",
			[]string{platformkafka.ExpiryTopic},
			notifications.NewDomainHandler(queue, clk), clk, logger, registry)
		if err != nil {
			return nil, nil, closeNotifications, err
		}
		errorReader, err := notifications.NewErrorReader(pool, cfg.Kafka, queue, clk, logger, registry)
		if err != nil {
			domainConsumer.Close()
			return nil, nil, closeNotifications, err
		}
		dispatcher := notifications.NewDispatcher(queue, notificationSender, clk, logger, registry)
		out = append(out, domainConsumer.Run, errorReader.Run, dispatcher.Run)
		closeNotifications = func() {
			domainConsumer.Close()
			errorReader.Close()
		}
	}

	return out, registry, closeNotifications, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load[appConfig]()
	if err != nil {
		return err
	}
	chatID, telegramEnabled, err := cfg.Telegram.resolve()
	if err != nil {
		return err
	}
	var sender notifications.Sender
	if telegramEnabled {
		sender, err = notifications.NewTelegramSender(cfg.Telegram.Token, chatID)
		if err != nil {
			return err
		}
	}
	logger := observability.NewLogger(os.Stdout, cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	producer, err := platformkafka.NewProducer(cfg.Kafka, "goden-worker-publisher")
	if err != nil {
		return err
	}
	defer producer.Close()

	background, registry, closeNotifications, err := tasks(pool, clock.System{}, logger, cfg, producer, sender)
	if err != nil {
		return err
	}
	defer closeNotifications()
	observability.RegisterPool(registry, pool)

	// сбой любой задачи останавливает процесс: оркестратор перезапустит его целиком
	group, ctx := errgroup.WithContext(ctx)
	for _, run := range background {
		group.Go(func() error { return run(ctx) })
	}
	mux := httpx.Health(pool.Ping)
	mux.Handle("GET /metrics", observability.MetricsHandler(registry))
	group.Go(func() error {
		return httpx.Serve(ctx, cfg.Addr, mux, cfg.ShutdownTimeout, logger)
	})
	return group.Wait()
}
