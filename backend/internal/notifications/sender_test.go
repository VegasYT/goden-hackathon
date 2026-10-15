package notifications

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type fakeTelegramBot struct {
	params *telegram.SendMessageParams
	err    error
}

func (b *fakeTelegramBot) SendMessage(_ context.Context, params *telegram.SendMessageParams) (*models.Message, error) {
	b.params = params
	return &models.Message{}, b.err
}

func TestTelegramSenderSendsPlainTextToConfiguredChat(t *testing.T) {
	client := &fakeTelegramBot{}
	sender := newTelegramSender(client, -1001234567890, "token")

	require.NoError(t, sender.Send(context.Background(), 0, "Тестовое уведомление"))
	require.Equal(t, int64(-1001234567890), client.params.ChatID)
	require.Equal(t, "Тестовое уведомление", client.params.Text)
	require.Empty(t, client.params.ParseMode)
}

func TestTelegramSenderRejectsEmptyMessageWithoutCallingClient(t *testing.T) {
	client := &fakeTelegramBot{}
	sender := newTelegramSender(client, 42, "token")

	require.Error(t, sender.Send(context.Background(), 0, ""))
	require.Nil(t, client.params)
}

// Создание отправителя не обращается к Telegram: с проверкой токена при старте недоступная
// сеть останавливала весь worker. Токен здесь заведомо неверный: проверка бы его отклонила.
func TestNewTelegramSenderDoesNotCallTelegram(t *testing.T) {
	sender, err := NewTelegramSender("0:not-a-real-token", 42)

	require.NoError(t, err)
	require.NotNil(t, sender)
}

// Причина сбоя возвращается, но без токена: ошибка сети содержит адрес запроса с токеном.
func TestTelegramSenderReportsCauseWithoutToken(t *testing.T) {
	const token = "123456:secret-token"
	client := &fakeTelegramBot{err: errors.New(`Post "https://api.telegram.org/bot` + token + `/sendMessage": forbidden, bot was kicked`)}
	sender := newTelegramSender(client, 42, token)

	err := sender.Send(context.Background(), 0, "текст")

	require.ErrorContains(t, err, "bot was kicked")
	require.NotContains(t, err.Error(), token)
	require.NotContains(t, err.Error(), "secret-token")
}

// Ответ "слишком часто" отправитель превращает в свою ошибку со временем ожидания:
// диспетчер откладывает по ней следующую попытку.
func TestTelegramSenderReportsRateLimit(t *testing.T) {
	client := &fakeTelegramBot{err: &telegram.TooManyRequestsError{Message: "Too Many Requests", RetryAfter: 30}}
	sender := newTelegramSender(client, 42, "token")

	err := sender.Send(context.Background(), 0, "текст")

	var limited *RateLimitedError
	require.ErrorAs(t, err, &limited)
	require.Equal(t, 30*time.Second, limited.RetryAfter)
	require.ErrorContains(t, err, "retry_after 30")
}

func TestLookupFailureIsPermanentOnlyWhenNothingIsFound(t *testing.T) {
	broken := errors.New("connection reset by peer")
	require.Equal(t, broken, lookupFailure(broken), "a connection failure is retried")

	missing := fmt.Errorf("notifications: product: %w", pgx.ErrNoRows)
	require.NotEqual(t, missing, lookupFailure(missing), "a missing product is wrapped as permanent")
}

// Чат точки заменяет общий чат окружения; ноль оставляет общий.
func TestTelegramSenderSendsToTheChatOfThePoint(t *testing.T) {
	client := &fakeTelegramBot{}
	sender := newTelegramSender(client, -1001234567890, "token")

	require.NoError(t, sender.Send(context.Background(), -1005, "Уведомление точки"))
	require.Equal(t, int64(-1005), client.params.ChatID)
}

// Отказ, который повтор не исправит, отличается от временного сбоя и от неверного токена.
func TestTelegramSenderTellsUndeliverableFromTemporary(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		undeliverable bool
	}{
		{"bot is not in the chat", fmt.Errorf("%w, Forbidden: bot is not a member of the channel chat", telegram.ErrorForbidden), true},
		{"chat not found", fmt.Errorf("%w, Bad Request: chat not found", telegram.ErrorBadRequest), true},
		{"not found", fmt.Errorf("%w, Not Found", telegram.ErrorNotFound), true},
		// неверный токен исправляет оператор: уведомления при этом не теряются
		{"wrong token", fmt.Errorf("%w, Unauthorized", telegram.ErrorUnauthorized), false},
		{"network", errors.New("dial tcp: i/o timeout"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := newTelegramSender(&fakeTelegramBot{err: tt.err}, -1001234567890, "token")
			err := sender.Send(context.Background(), -1005, "текст")
			require.Error(t, err)
			var undeliverable *UndeliverableError
			require.Equal(t, tt.undeliverable, errors.As(err, &undeliverable))
		})
	}
}
