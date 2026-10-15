package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/platform/postgres"
)

const (
	consumerMaxBackoff = 5 * time.Second
	// defaultMaxAttempts - сколько раз обрабатывается одно событие, прежде чем уйти в поток ошибок.
	// С паузой до 5 секунд это около 40 секунд: короткий сбой базы событие переживает,
	// а постоянная ошибка не держит раздел без конца.
	defaultMaxAttempts = 10
)

// Handler меняет свои таблицы через транзакцию inbox. Его изменения и отметка event_id
// фиксируются атомарно, поэтому повторная доставка не вызывает обработку второй раз.
type Handler func(context.Context, pgx.Tx, Envelope) error

type permanentError struct{ error }

// Permanent помечает ошибку данных, которую повтор не исправит. Consumer публикует её
// в errors topic и подтверждает исходное сообщение. Обычная ошибка повторяется с backoff.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{error: err}
}

func isPermanent(err error) bool {
	var target permanentError
	return errors.As(err, &target)
}

// Consumer - каркас Kafka consumer group с durable inbox.
type Consumer struct {
	pool     *pgxpool.Pool
	client   *kgo.Client
	producer Producer
	handler  Handler
	clock    clock.Clock
	logger   *slog.Logger
	name     string
	prefix   string

	// MaxAttempts - предел попыток обработки одного события; ноль означает значение по умолчанию.
	MaxAttempts int
	topics      []string

	processed *prometheus.CounterVec
	failures  prometheus.Counter
}

func NewConsumer(pool *pgxpool.Pool, cfg Config, name string, topics []string, handler Handler,
	clk clock.Clock, logger *slog.Logger, registry prometheus.Registerer,
) (*Consumer, error) {
	if pool == nil || len(cfg.Brokers) == 0 || name == "" || len(topics) == 0 || handler == nil || clk == nil || logger == nil {
		return nil, errors.New("kafka consumer: invalid configuration")
	}
	fullTopics := make([]string, len(topics))
	for i, topic := range topics {
		fullTopics[i] = fullTopic(cfg.TopicPrefix, topic)
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID("goden-"+name),
		kgo.ConsumerGroup("goden-"+name),
		kgo.ConsumeTopics(fullTopics...),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(produceTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: create client: %w", err)
	}
	return &Consumer{
		pool: pool, client: client, producer: &Client{inner: client, known: make(map[string]struct{})}, handler: handler,
		clock: clk, logger: logger, name: name, prefix: cfg.TopicPrefix, topics: fullTopics,
		processed: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "kafka_consumer_messages_total",
			Help: "Kafka messages completed by consumer and outcome.",
		}, []string{"consumer", "outcome"})),
		failures: observability.Register(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "kafka_consumer_errors_total",
			Help: "Retryable Kafka consumer handler failures.",
		})),
	}, nil
}

func (c *Consumer) Close() { c.client.CloseAllowingRebalance() }

// Run получает по одному сообщению: пока оно не обработано, следующая запись этой
// consumer group не подтверждается. Это сохраняет порядок и делает retry явным.
//
// Сбой Kafka не завершает задачу: вернувшаяся ошибка остановила бы весь процесс вместе
// с проекциями и контролем сроков. Обработчик пишет ошибку и повторяет с паузой.
func (c *Consumer) Run(ctx context.Context) error {
	var backoff time.Duration
	for {
		err := EnsureTopics(ctx, c.client, c.topics)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			break
		}
		if !c.infrastructureFailure(ctx, &backoff, "ensure topics", err) {
			return nil
		}
	}
	backoff = 0
	for {
		fetches := c.client.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			c.client.AllowRebalance()
			if !c.infrastructureFailure(ctx, &backoff, "fetch", errs[0].Err) {
				return nil
			}
			continue
		}
		backoff = 0
		for _, record := range fetches.Records() {
			if err := c.processWithRetry(ctx, record); err != nil {
				c.client.AllowRebalance()
				return err
			}
			if ctx.Err() != nil {
				c.client.AllowRebalance()
				return nil
			}
			// неподтверждённое сообщение будет прочитано снова и отсечено отметкой об обработке
			if err := c.client.CommitRecords(ctx, record); err != nil && ctx.Err() == nil {
				c.failures.Inc()
				c.logger.ErrorContext(ctx, "Kafka consumer failed to commit offset", "consumer", c.name, "error", err)
			}
			c.client.AllowRebalance()
		}
	}
}

// infrastructureFailure записывает сбой Kafka и ждёт перед повтором; false - контекст отменён.
func (c *Consumer) infrastructureFailure(ctx context.Context, backoff *time.Duration, action string, err error) bool {
	c.failures.Inc()
	*backoff = min(max(*backoff*2, publisherMinBackoff), consumerMaxBackoff)
	c.logger.ErrorContext(ctx, "Kafka consumer failed", "consumer", c.name, "action", action,
		"retry_in", backoff.String(), "error", err)
	return wait(ctx, *backoff)
}

func (c *Consumer) maxAttempts() int {
	if c.MaxAttempts > 0 {
		return c.MaxAttempts
	}
	return defaultMaxAttempts
}

func (c *Consumer) processWithRetry(ctx context.Context, record *kgo.Record) error {
	backoff := time.Duration(0)
	for attempt := 1; ; attempt++ {
		outcome, err := c.Process(ctx, record.Topic, record.Value)
		if err == nil {
			c.processed.WithLabelValues(c.name, outcome).Inc()
			return nil
		}
		c.failures.Inc()
		if attempt >= c.maxAttempts() {
			// попытки исчерпаны: событие уходит в поток ошибок, раздел не стоит без конца.
			// Если отметить это не удалось (например, база недоступна), повторы продолжаются.
			if outcome, giveUpErr := c.giveUp(ctx, record, err); giveUpErr == nil {
				c.processed.WithLabelValues(c.name, outcome).Inc()
				return nil
			}
		}
		backoff = min(max(backoff*2, publisherMinBackoff), consumerMaxBackoff)
		c.logger.ErrorContext(ctx, "Kafka consumer handler failed", "consumer", c.name,
			"event_topic", record.Topic, "retry_in", backoff.String(), "error", err)
		if !wait(ctx, backoff) {
			return nil
		}
	}
}

// giveUp отправляет событие, которое не удалось обработать за все попытки, в поток ошибок.
func (c *Consumer) giveUp(ctx context.Context, record *kgo.Record, cause error) (string, error) {
	var event Envelope
	_ = json.Unmarshal(record.Value, &event) // конверт уже разбирался в Process; здесь нужны только идентификаторы
	return c.sendMalformed(ctx, record.Topic, record.Value, event.EventID, event.EventType,
		fmt.Errorf("gave up after %d attempts: %w", c.maxAttempts(), cause))
}

// Process выполняет одну доставку и возвращает handled, duplicate или error-stream.
// Метод полезен адаптерам и тестам, которым цикл чтения franz-go не нужен.
func (c *Consumer) Process(ctx context.Context, sourceTopic string, raw []byte) (string, error) {
	var event Envelope
	if err := json.Unmarshal(raw, &event); err != nil {
		return c.sendMalformed(ctx, sourceTopic, raw, uuid.Nil, "", fmt.Errorf("decode envelope: %w", err))
	}
	if event.EnvelopeVersion != EnvelopeVersion || event.EventID == uuid.Nil || event.EventType == "" {
		return c.sendMalformed(ctx, sourceTopic, raw, event.EventID, event.EventType,
			fmt.Errorf("unsupported envelope_version %d or empty event_id/event_type", event.EnvelopeVersion))
	}

	duplicate := false
	err := postgres.InTx(ctx, c.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM kafka_consumed_events WHERE consumer_name = $1 AND event_id = $2
		)`, c.name, event.EventID).Scan(&exists); err != nil {
			return fmt.Errorf("read inbox: %w", err)
		}
		if exists {
			duplicate = true
			return nil
		}
		if err := c.handler(ctx, tx, event); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO kafka_consumed_events (consumer_name, event_id, outcome, processed_at)
			VALUES ($1, $2, 'handled', $3)`, c.name, event.EventID, c.clock.Now())
		if err != nil {
			return fmt.Errorf("write inbox: %w", err)
		}
		return nil
	})
	if err == nil {
		if duplicate {
			return "duplicate", nil
		}
		return "handled", nil
	}
	if !isPermanent(err) {
		return "", fmt.Errorf("kafka consumer %s: handle %s: %w", c.name, event.EventID, err)
	}
	return c.sendMalformed(ctx, sourceTopic, raw, event.EventID, event.EventType, err)
}

func (c *Consumer) sendMalformed(ctx context.Context, sourceTopic string, raw []byte,
	eventID uuid.UUID, eventType string, cause error,
) (string, error) {
	errorEvent := ErrorEnvelope{
		Consumer: c.name, SourceTopic: sourceTopic, EventID: eventID, EventType: eventType,
		Error: cause.Error(), Original: raw, FailedAt: c.clock.Now(),
	}
	encoded, err := json.Marshal(errorEvent)
	if err != nil {
		return "", fmt.Errorf("kafka consumer %s: encode error: %w", c.name, err)
	}
	key := []byte(eventID.String())
	if eventID == uuid.Nil {
		key = nil
	}
	if err := c.producer.Produce(ctx, Message{
		Topic: fullTopic(c.prefix, ErrorsTopic), Key: key, Value: encoded,
	}); err != nil {
		return "", fmt.Errorf("kafka consumer %s: publish error: %w", c.name, err)
	}
	if eventID != uuid.Nil {
		_, err = c.pool.Exec(ctx, `
			INSERT INTO kafka_consumed_events (consumer_name, event_id, outcome, processed_at)
			VALUES ($1, $2, 'error', $3)
			ON CONFLICT (consumer_name, event_id) DO NOTHING`, c.name, eventID, c.clock.Now())
		if err != nil {
			return "", fmt.Errorf("kafka consumer %s: save error outcome: %w", c.name, err)
		}
	}
	return "error-stream", nil
}
