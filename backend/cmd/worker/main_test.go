package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/notifications"
	"goden/backend/internal/platform/clock"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/readmodel"
)

func TestTelegramConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  telegramConfig
		chatID  int64
		enabled bool
		errText string
	}{
		{name: "disabled"},
		{name: "enabled", config: telegramConfig{Token: "secret-token", ChatID: "-100123"}, chatID: -100123, enabled: true},
		{name: "missing chat", config: telegramConfig{Token: "secret-token"}, errText: "TELEGRAM_CHAT_ID"},
		{name: "missing token", config: telegramConfig{ChatID: "42"}, errText: "TELEGRAM_BOT_TOKEN"},
		{name: "invalid chat", config: telegramConfig{Token: "secret-token", ChatID: "not-a-number"}, errText: "TELEGRAM_CHAT_ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatID, enabled, err := tt.config.resolve()
			if tt.errText != "" {
				require.ErrorContains(t, err, tt.errText)
				require.NotContains(t, err.Error(), "secret-token")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.chatID, chatID)
			require.Equal(t, tt.enabled, enabled)
		})
	}
}

type noopSender struct{}

func (noopSender) Send(context.Context, int64, string) error { return nil }

func TestTasksAddNotificationsOnlyWhenSenderIsConfigured(t *testing.T) {
	pool := new(pgxpool.Pool)
	cfg := appConfig{
		Kafka:               platformkafka.Config{Brokers: []string{"localhost:1"}, TopicPrefix: "test"},
		ExpiryWarningPeriod: time.Hour, ExpiryCheckInterval: time.Minute,
	}
	base := len(readmodel.Projections()) + 1

	disabled, _, closeDisabled, err := tasks(pool, clock.System{}, slog.New(slog.DiscardHandler), cfg, nil, nil)
	require.NoError(t, err)
	t.Cleanup(closeDisabled)
	require.Len(t, disabled, base)

	enabled, _, closeEnabled, err := tasks(pool, clock.System{}, slog.New(slog.DiscardHandler), cfg, nil, notifications.Sender(noopSender{}))
	require.NoError(t, err)
	t.Cleanup(closeEnabled)
	require.Len(t, enabled, base+3)
}

func TestRunRejectsPartialTelegramBeforeExternalConnections(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://unreachable")
	t.Setenv("TELEGRAM_BOT_TOKEN", "secret-token")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	err := run()
	require.ErrorContains(t, err, "TELEGRAM_CHAT_ID")
	require.NotContains(t, err.Error(), "secret-token")
}
