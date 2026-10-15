// Команда migrate применяет миграции к базе из переменной окружения DATABASE_URL.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"goden/backend/internal/demo"
	"goden/backend/internal/generator"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres"
)

type appConfig struct {
	Postgres config.Postgres
	Log      config.Log
	Demo     demoConfig
}

// demoConfig - демонстрационная компания с тестовыми пользователями realm (internal/demo).
type demoConfig struct {
	// Enabled создаёт компанию после миграций; повторный запуск ничего не добавляет.
	Enabled bool `env:"DEMO_COMPANY" envDefault:"false"`
	// TelegramChatID - чат Telegram первой точки; пусто или ноль - чат не задаётся. Строка,
	// а не число: опечатка в необязательной настройке не должна останавливать миграции.
	TelegramChatID string `env:"DEMO_TELEGRAM_CHAT_ID" envDefault:"0"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load[appConfig]()
	if err != nil {
		return err
	}
	logger := observability.NewLogger(os.Stdout, cfg.Log.Level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "migrations applied", "count", len(applied), "files", applied)

	if cfg.Demo.Enabled {
		clk := clock.System{}
		org := organization.NewService(command.NewRunner(pool, clk, logger), pool)
		chatID, err := strconv.ParseInt(strings.TrimSpace(cfg.Demo.TelegramChatID), 10, 64)
		if err != nil && strings.TrimSpace(cfg.Demo.TelegramChatID) != "" {
			logger.WarnContext(ctx, "DEMO_TELEGRAM_CHAT_ID is not a number, the demo point gets no chat")
		}
		// Демонстрационная компания необязательна: её сбой не должен останавливать запуск системы,
		// которая ждёт завершения миграций. Миграции к этому моменту уже применены.
		if err := demo.Seed(ctx, org, generator.NewService(pool, clk), chatID); err != nil {
			logger.ErrorContext(ctx, "demo company is not ready", "error", err)
			return nil
		}
		logger.InfoContext(ctx, "demo company is ready", "company", demo.CompanyName)
	}
	return nil
}
