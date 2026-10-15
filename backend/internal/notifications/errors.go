package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"

	"goden/backend/internal/platform/clock"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres"
)

const errorReaderName = "notifications-errors"

// ErrorReader читает отдельный формат errors.v1 и никогда не пишет ошибку обратно в него.
type ErrorReader struct {
	pool   *pgxpool.Pool
	client *kgo.Client
	queue  *Queue
	clock  clock.Clock
	logger *slog.Logger
	topic  string

	malformed prometheus.Counter
}

func NewErrorReader(pool *pgxpool.Pool, cfg platformkafka.Config, queue *Queue, clk clock.Clock,
	logger *slog.Logger, registry prometheus.Registerer,
) (*ErrorReader, error) {
	if pool == nil || queue == nil || len(cfg.Brokers) == 0 || clk == nil || logger == nil || registry == nil {
		return nil, errors.New("notification error reader: invalid configuration")
	}
	topic := platformkafka.FullTopic(cfg.TopicPrefix, platformkafka.ErrorsTopic)
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID("goden-"+errorReaderName),
		kgo.ConsumerGroup("goden-"+errorReaderName),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		return nil, fmt.Errorf("notification error reader: create client: %w", err)
	}
	return &ErrorReader{
		pool: pool, client: client, queue: queue, clock: clk, logger: logger, topic: topic,
		malformed: observability.Register(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "notification_error_reader_errors_total",
			Help: "Malformed messages skipped by the notification error reader.",
		})),
	}, nil
}

func (r *ErrorReader) Close() { r.client.CloseAllowingRebalance() }

// Run читает поток ошибок, пока не отменён ctx. Сбой Kafka или базы не завершает задачу:
// вернувшаяся ошибка остановила бы весь worker вместе с проекциями.
func (r *ErrorReader) Run(ctx context.Context) error {
	for {
		err := platformkafka.EnsureTopics(ctx, r.client, []string{r.topic})
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			break
		}
		if !r.failure(ctx, "ensure topic", err) {
			return nil
		}
	}
	for {
		fetches := r.client.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			r.client.AllowRebalance()
			if !r.failure(ctx, "fetch", errs[0].Err) {
				return nil
			}
			continue
		}
		for _, record := range fetches.Records() {
			for {
				_, err := r.Process(ctx, record.Value)
				if err == nil || ctx.Err() != nil {
					break
				}
				if !r.failure(ctx, "enqueue", err) {
					break
				}
			}
			if ctx.Err() != nil {
				r.client.AllowRebalance()
				return nil
			}
			// неподтверждённая запись будет прочитана снова и отсечена ключом очереди
			if err := r.client.CommitRecords(ctx, record); err != nil && ctx.Err() == nil {
				r.logger.ErrorContext(ctx, "Notification error reader failed to commit offset", "error", err)
			}
			r.client.AllowRebalance()
		}
	}
}

// failure записывает сбой и ждёт перед повтором; false - контекст отменён.
func (r *ErrorReader) failure(ctx context.Context, action string, err error) bool {
	r.logger.ErrorContext(ctx, "Notification error reader failed", "action", action,
		"retry_in", errorReaderRetryDelay.String(), "error", err)
	return waitFor(ctx, errorReaderRetryDelay)
}

// Process сохраняет уведомление или безопасно пропускает некорректный ErrorEnvelope.
func (r *ErrorReader) Process(ctx context.Context, raw []byte) (string, error) {
	delivery, failedAt, ok := errorDelivery(raw)
	if !ok {
		r.malformed.Inc()
		r.logger.ErrorContext(ctx, "Notification error reader skipped malformed message")
		return "malformed", nil
	}
	if stale(r.clock.Now(), failedAt) {
		return "stale", nil
	}
	if err := postgres.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		return r.queue.Enqueue(ctx, tx, delivery, r.clock.Now())
	}); err != nil {
		return "", fmt.Errorf("notification error reader: enqueue: %w", err)
	}
	return "handled", nil
}

// errorReaderRetryDelay - пауза перед повтором после сбоя Kafka или базы.
const errorReaderRetryDelay = time.Second

// errorDelivery собирает уведомление и возвращает время сбоя из записи.
func errorDelivery(raw []byte) (Delivery, time.Time, bool) {
	var event platformkafka.ErrorEnvelope
	if err := json.Unmarshal(raw, &event); err != nil || event.Consumer == "" || event.SourceTopic == "" || event.Error == "" {
		return Delivery{}, time.Time{}, false
	}
	// Ключ по событию, а не по содержимому записи: в записи есть время сбоя, и повторная запись
	// об одном событии давала второе уведомление. Запись без события различается содержимым.
	key := event.Consumer + ":" + event.EventID.String()
	if event.EventID == uuid.Nil {
		digest := sha256.Sum256(raw)
		key = hex.EncodeToString(digest[:])
	}
	return Delivery{
		SourceKey: "consumer-error:" + key,
		Kind:      KindConsumerError,
		Body:      consumerErrorText(event),
	}, event.FailedAt, true
}

func consumerErrorText(event platformkafka.ErrorEnvelope) string {
	eventID := event.EventID.String()
	if event.EventID.String() == "00000000-0000-0000-0000-000000000000" {
		eventID = "не указан"
	}
	eventType := event.EventType
	if eventType == "" {
		eventType = "не указан"
	}
	return fmt.Sprintf("🚨 Ошибка фонового обработчика\nОбработчик: %s\nТопик: %s\nТип события: %s\nСобытие: %s\nОшибка: %s",
		event.Consumer, event.SourceTopic, eventType, eventID, safeError(event.Error))
}

func safeError(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r < 0x20 {
			return ' '
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	const maxRunes = 500
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes]) + "…"
	}
	return value
}
