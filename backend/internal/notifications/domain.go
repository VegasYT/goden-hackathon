package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/clock"
	platformkafka "goden/backend/internal/platform/kafka"
)

// maxEventAge - старше этого возраста событие уведомления не порождает. Новая группа обработчика
// читает топики с начала, поэтому без отсечки при первом включении и после простоя в чат
// уходила вся история. Следствие: уведомления за простой дольше часа не приходят.
const maxEventAge = time.Hour

// stale сообщает, что событие слишком старое для уведомления. Событие без времени не отсекается.
func stale(now, occurredAt time.Time) bool {
	return !occurredAt.IsZero() && now.Sub(occurredAt) > maxEventAge
}

// NewDomainHandler создаёт обработчик доменных Kafka-конвертов для durable inbox.
func NewDomainHandler(queue *Queue, clk clock.Clock) platformkafka.Handler {
	return func(ctx context.Context, tx pgx.Tx, event platformkafka.Envelope) error {
		return handleDomain(ctx, tx, queue, clk, event)
	}
}

func handleDomain(ctx context.Context, tx pgx.Tx, queue *Queue, clk clock.Clock, event platformkafka.Envelope) error {
	if queue == nil || clk == nil {
		return fmt.Errorf("notifications: invalid domain handler configuration")
	}
	// пропуск - успешная обработка: событие отмечается обработанным и не повторяется
	if stale(clk.Now(), event.OccurredAt) {
		return nil
	}
	switch event.EventType {
	case "BatchExpiryWarningIssued":
		if event.EventVersion != 1 {
			return platformkafka.Permanent(fmt.Errorf("notifications: unsupported BatchExpiryWarningIssued version %d", event.EventVersion))
		}
		var payload inventory.BatchExpiryWarningIssued
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return platformkafka.Permanent(fmt.Errorf("notifications: decode BatchExpiryWarningIssued: %w", err))
		}
		if payload.ProductID == uuid.Nil || payload.BatchID == uuid.Nil || payload.RemainingQuantity < 0 || payload.ExpiresAt.IsZero() {
			return platformkafka.Permanent(fmt.Errorf("notifications: invalid BatchExpiryWarningIssued payload"))
		}
		if !payload.ExpiresAt.After(clk.Now()) {
			return nil // срок уже истёк: предупреждать не о чем, о просрочке сообщит BatchExpired
		}
		// точка читается первой: без чата остальное читать незачем, и сбой чтения продукта не шумит
		pointName, chatID, err := pointTarget(ctx, tx, payload.PointID)
		if err != nil {
			return err
		}
		if chatID == 0 {
			return nil // у точки нет чата: уведомлять некого
		}
		name, unit, err := productName(ctx, tx, payload.ProductID)
		if err != nil {
			return lookupFailure(err)
		}
		return queue.Enqueue(ctx, tx, Delivery{
			SourceKey: event.EventID.String(), Kind: KindExpiryWarning, ChatID: chatID,
			Body: withPoint(expiryWarningText(name, payload.BatchID, quantityText(payload.RemainingQuantity, unit), payload.ExpiresAt.Time), pointName),
		}, clk.Now())

	case "BatchExpired":
		if event.EventVersion != 1 {
			return platformkafka.Permanent(fmt.Errorf("notifications: unsupported BatchExpired version %d", event.EventVersion))
		}
		var payload inventory.BatchExpired
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return platformkafka.Permanent(fmt.Errorf("notifications: decode BatchExpired: %w", err))
		}
		if payload.ProductID == uuid.Nil || payload.BatchID == uuid.Nil || payload.RemainingQuantity <= 0 || payload.ExpiresAt.IsZero() {
			return platformkafka.Permanent(fmt.Errorf("notifications: invalid BatchExpired payload"))
		}
		// точка читается первой: без чата остальное читать незачем, и сбой чтения продукта не шумит
		pointName, chatID, err := pointTarget(ctx, tx, payload.PointID)
		if err != nil {
			return err
		}
		if chatID == 0 {
			return nil // у точки нет чата: уведомлять некого
		}
		name, unit, err := productName(ctx, tx, payload.ProductID)
		if err != nil {
			return lookupFailure(err)
		}
		return queue.Enqueue(ctx, tx, Delivery{
			SourceKey: event.EventID.String(), Kind: KindBatchExpired, ChatID: chatID,
			Body: withPoint(batchExpiredText(name, payload.BatchID, quantityText(payload.RemainingQuantity, unit), payload.ExpiresAt.Time), pointName),
		}, clk.Now())
	default:
		return nil
	}
}

// lookupFailure решает, окончательна ли ошибка чтения. Повтор не исправит только отсутствие
// продукта или партии; сбой соединения повторяется, иначе уведомление терялось бы навсегда.
func lookupFailure(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return platformkafka.Permanent(err)
	}
	return err
}

// quantityText пишет количество с единицей измерения продукта: число без единицы в сообщении
// не отличить - 400 граммов или 400 штук.
func quantityText(quantity int64, unit string) string {
	label, known := map[string]string{"g": "г", "ml": "мл", "pcs": "шт"}[unit]
	if !known {
		return fmt.Sprint(quantity)
	}
	return fmt.Sprintf("%d %s", quantity, label)
}

// pointTarget возвращает название точки события и её чат Telegram. Чат определяется здесь,
// при постановке в очередь: запись очереди хранит своего адресата. Ноль - у точки чата нет
// (или точки нет, как у события без точки): такое уведомление не создаётся. В общий чат
// окружения оно не уходит: там оказались бы данные точек чужих компаний.
func pointTarget(ctx context.Context, tx pgx.Tx, pointID uuid.UUID) (name string, chatID int64, err error) {
	err = tx.QueryRow(ctx, `SELECT name, COALESCE(telegram_chat_id, 0) FROM organization_points WHERE id = $1`, pointID).
		Scan(&name, &chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("notifications: point %s: %w", pointID, err)
	}
	return name, chatID, nil
}

// withPoint дописывает к тексту уведомления точку, на которой произошло событие.
func withPoint(text, pointName string) string {
	if pointName == "" {
		return text
	}
	return text + "\nТочка: " + pointName
}

func productName(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (name, unit string, err error) {
	if err := tx.QueryRow(ctx, `SELECT name, unit FROM catalog_products WHERE id = $1`, productID).Scan(&name, &unit); err != nil {
		return "", "", fmt.Errorf("notifications: product %s: %w", productID, err)
	}
	return name, unit, nil
}

func expiryWarningText(product string, batchID uuid.UUID, quantity string, expiresAt time.Time) string {
	return fmt.Sprintf("⚠️ Срок годности скоро истечёт\nПродукт: %s\nПартия: %s\nОстаток: %s\nГоден до: %s",
		product, batchID, quantity, expiresAt.UTC().Format(timeLayout))
}

// timeLayout - время в тексте уведомления. Часового пояса у точки нет, поэтому UTC назван явно.
const timeLayout = "02.01.2006 15:04 UTC"

func batchExpiredText(product string, batchID uuid.UUID, quantity string, expiresAt time.Time) string {
	return fmt.Sprintf("❌ Партия просрочена: использовать нельзя, нужно решение о списании\nПродукт: %s\nПартия: %s\nОстаток: %s\nСрок годности: %s",
		product, batchID, quantity, expiresAt.UTC().Format(timeLayout))
}
