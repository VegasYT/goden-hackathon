package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
)

const (
	PublisherName             = "domain-events"
	defaultPublisherBatchSize = 200
	defaultPublisherInterval  = 200 * time.Millisecond
	publisherMinBackoff       = 100 * time.Millisecond
	// leaderRetryInterval - как часто ожидающий экземпляр проверяет, не освободилась ли блокировка
	leaderRetryInterval = time.Second
	publisherMaxBackoff = 5 * time.Second
)

// Publisher читает Event Store независимо от доменных команд. Сбой Kafka не откатывает
// бизнес-операцию: неподтверждённое событие остаётся за checkpoint и будет повторено.
type Publisher struct {
	pool     *pgxpool.Pool
	producer Producer
	clock    clock.Clock
	logger   *slog.Logger
	prefix   string

	BatchSize    int
	PollInterval time.Duration

	lag       prometheus.Gauge
	published *prometheus.CounterVec
	failures  prometheus.Counter
}

func NewPublisher(pool *pgxpool.Pool, producer Producer, clk clock.Clock, logger *slog.Logger,
	registry prometheus.Registerer, prefix string,
) *Publisher {
	return &Publisher{
		pool: pool, producer: producer, clock: clk, logger: logger, prefix: prefix,
		BatchSize: defaultPublisherBatchSize, PollInterval: defaultPublisherInterval,
		lag: observability.Register(registry, prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: observability.MetricsNamespace, Name: "kafka_publisher_lag_events",
			Help: "Event Store events not yet inspected by the Kafka publisher.",
		})),
		published: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "kafka_published_events_total",
			Help: "Integration events acknowledged by Kafka by topic.",
		}, []string{"topic"})),
		failures: observability.Register(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "kafka_publisher_errors_total",
			Help: "Failed publisher iterations; the event remains pending for retry.",
		})),
	}
}

func (p *Publisher) Run(ctx context.Context) error {
	if p.pool == nil || p.producer == nil || p.clock == nil || p.logger == nil || p.BatchSize <= 0 || p.PollInterval <= 0 {
		return errors.New("kafka publisher: invalid configuration")
	}
	var backoff time.Duration
	for {
		progressed, err := p.lead(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if progressed {
			backoff = 0
		}
		pause := leaderRetryInterval // блокировку держит другой экземпляр
		if err != nil {
			p.failures.Inc()
			backoff = min(max(backoff*2, publisherMinBackoff), publisherMaxBackoff)
			pause = backoff
			p.logger.ErrorContext(ctx, "Kafka publisher failed", "retry_in", backoff.String(), "error", err)
			p.observeLag(ctx)
		}
		if !wait(ctx, pause) {
			return nil
		}
	}
}

// lead публикует, пока этот экземпляр держит блокировку единственного publisher. Возвращает
// nil, если блокировку держит другой экземпляр; progressed - была ли опубликована хотя бы порция.
//
// Блокировка сессионная, на отдельном соединении. Транзакция для этого не годится: открытая на
// время обращения к Kafka, она задерживала бы всех читателей Event Store (ADR-002).
func (p *Publisher) lead(ctx context.Context) (progressed bool, err error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("kafka publisher: acquire lock connection: %w", err)
	}
	// соединение закрывается, а не возвращается в пул: иначе блокировка осталась бы у пула
	defer func() { _ = conn.Hijack().Close(context.WithoutCancel(ctx)) }()

	var leader bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, "kafka-publisher:"+PublisherName).Scan(&leader); err != nil {
		return false, fmt.Errorf("kafka publisher: take lock: %w", err)
	}
	if !leader {
		return false, nil
	}
	for {
		// потерянное соединение означает снятую блокировку: публиковать дальше нельзя
		if err := conn.Ping(ctx); err != nil {
			return progressed, fmt.Errorf("kafka publisher: lock connection: %w", err)
		}
		n, err := p.PublishBatch(ctx)
		if err != nil {
			return progressed, err
		}
		progressed = true
		// после каждой порции: при разборе большого отставания метрика не должна стоять
		p.observeLag(ctx)
		if n < p.BatchSize && !wait(ctx, p.PollInterval) {
			return progressed, nil
		}
	}
}

// PublishBatch публикует одну порцию. Метод экспортирован для детерминированных тестов
// восстановления: production-цикл вызывает его с backoff.
func (p *Publisher) PublishBatch(ctx context.Context) (int, error) {
	checkpoint, err := p.checkpoint(ctx)
	if err != nil {
		return 0, err
	}
	events, err := eventstore.ReadAll(ctx, p.pool, checkpoint, p.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("kafka publisher: read events: %w", err)
	}
	for i, event := range events {
		message, publish, err := marshalEvent(event, p.prefix)
		if err != nil {
			return i, err
		}
		if publish {
			if err := p.producer.Produce(ctx, message); err != nil {
				return i, fmt.Errorf("kafka publisher: event %s: %w", event.ID, err)
			}
			p.published.WithLabelValues(message.Topic).Inc()
		}
		if err := p.saveCheckpoint(ctx, event.Checkpoint()); err != nil {
			return i, err
		}
	}
	return len(events), nil
}

func (p *Publisher) checkpoint(ctx context.Context) (eventstore.Checkpoint, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT txid::text, position FROM kafka_publisher_checkpoints WHERE name = $1`, PublisherName)
	var txid string
	var checkpoint eventstore.Checkpoint
	if err := row.Scan(&txid, &checkpoint.Position); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return checkpoint, nil
		}
		return checkpoint, fmt.Errorf("kafka publisher: read checkpoint: %w", err)
	}
	parsed, err := strconv.ParseUint(txid, 10, 64)
	if err != nil {
		return checkpoint, fmt.Errorf("kafka publisher: parse checkpoint txid: %w", err)
	}
	checkpoint.TxID = parsed
	return checkpoint, nil
}

func (p *Publisher) saveCheckpoint(ctx context.Context, checkpoint eventstore.Checkpoint) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO kafka_publisher_checkpoints (name, txid, position, updated_at)
		VALUES ($1, $2::text::xid8, $3, $4)
		ON CONFLICT (name) DO UPDATE
		SET txid = EXCLUDED.txid, position = EXCLUDED.position, updated_at = EXCLUDED.updated_at
		WHERE (kafka_publisher_checkpoints.txid, kafka_publisher_checkpoints.position)
		    < (EXCLUDED.txid, EXCLUDED.position)`,
		PublisherName, strconv.FormatUint(checkpoint.TxID, 10), checkpoint.Position, p.clock.Now())
	if err != nil {
		return fmt.Errorf("kafka publisher: save checkpoint: %w", err)
	}
	return nil
}

func (p *Publisher) observeLag(ctx context.Context) {
	checkpoint, err := p.checkpoint(ctx)
	if err != nil {
		return
	}
	row := p.pool.QueryRow(ctx, `
		SELECT count(*) FROM events WHERE (txid, position) > ($1::text::xid8, $2)`,
		strconv.FormatUint(checkpoint.TxID, 10), checkpoint.Position)
	var lag float64
	if row.Scan(&lag) == nil {
		p.lag.Set(lag)
	}
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
