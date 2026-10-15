package notifications

import (
	"context"
	"errors"
	"strings"
	"time"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// sendTimeout - срок на одно обращение к Telegram. Он короче времени, на которое диспетчер
// захватывает уведомление: клиент библиотеки ждёт до минуты, и за это время второй экземпляр
// worker забирал запись и отправлял сообщение повторно.
const sendTimeout = 10 * time.Second

// RateLimitedError - Telegram отклонил сообщение как слишком частое и назвал время ожидания.
type RateLimitedError struct {
	RetryAfter time.Duration
	cause      string
}

func (e *RateLimitedError) Error() string { return e.cause }

// UndeliverableError - Telegram отклонил сообщение так, что повтор этого не исправит: чат
// не найден, бот не состоит в чате, запрос отклонён. Чат точки задаёт пользователь, поэтому
// такое уведомление не повторяется: иначе один неверный чат занимал бы диспетчер всех точек.
type UndeliverableError struct{ cause string }

func (e *UndeliverableError) Error() string { return e.cause }

// Sender скрывает Telegram SDK от очереди и позволяет тестировать доставку без сети.
// chatID - чат точки; ноль - общий чат окружения, в него уходят только ошибки обработчиков.
type Sender interface {
	Send(ctx context.Context, chatID int64, text string) error
}

type telegramBot interface {
	SendMessage(context.Context, *telegram.SendMessageParams) (*models.Message, error)
}

type TelegramSender struct {
	bot    telegramBot
	chatID int64
	token  string // только чтобы убирать его из текста ошибок
}

func NewTelegramSender(token string, chatID int64) (*TelegramSender, error) {
	if token == "" || chatID == 0 {
		return nil, errors.New("telegram sender: invalid configuration")
	}
	// Без WithSkipGetMe библиотека при создании обращается к Telegram, и недоступная сеть или прокси
	// не давали запуститься всему worker вместе с проекциями. Неверный токен теперь виден
	// по ошибкам отправки.
	client, err := telegram.New(token, telegram.WithSkipGetMe())
	if err != nil {
		return nil, errors.New("telegram sender: initialize bot: " + maskToken(err.Error(), token))
	}
	return newTelegramSender(client, chatID, token), nil
}

func newTelegramSender(client telegramBot, chatID int64, token string) *TelegramSender {
	return &TelegramSender{bot: client, chatID: chatID, token: token}
}

// maskToken убирает токен из текста ошибки: библиотека скрывает его не во всех путях.
func maskToken(text, token string) string {
	if token == "" {
		return text
	}
	return strings.ReplaceAll(text, token, "***")
}

func (s *TelegramSender) Send(ctx context.Context, chatID int64, text string) error {
	if s == nil || s.bot == nil || s.chatID == 0 || text == "" {
		return errors.New("telegram sender: invalid message")
	}
	if chatID == 0 {
		chatID = s.chatID
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	_, err := s.bot.SendMessage(ctx, &telegram.SendMessageParams{ChatID: chatID, Text: text})
	if err != nil {
		// текст вместо обёртки: исходная ошибка может содержать токен в адресе запроса
		cause := "telegram sender: send message: " + maskToken(err.Error(), s.token)
		var tooMany *telegram.TooManyRequestsError
		if errors.As(err, &tooMany) {
			return &RateLimitedError{RetryAfter: time.Duration(tooMany.RetryAfter) * time.Second, cause: cause}
		}
		// неверный токен (unauthorized) сюда не входит: это настройка окружения, её исправляет
		// оператор, и уведомления при этом теряться не должны
		if errors.Is(err, telegram.ErrorForbidden) || errors.Is(err, telegram.ErrorBadRequest) || errors.Is(err, telegram.ErrorNotFound) {
			return &UndeliverableError{cause: cause}
		}
		return errors.New(cause)
	}
	return nil
}
