package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	platformkafka "goden/backend/internal/platform/kafka"
)

const readerRetryDelay = time.Second

// Reader независимо читает интеграционные события для локального SSE hub. У него
// нет consumer group: каждый экземпляр API должен уведомить собственных клиентов.
type Reader struct {
	client *kgo.Client
	hub    *Hub
	logger *slog.Logger
	topics []string
}

func NewReader(cfg platformkafka.Config, hub *Hub, logger *slog.Logger) (*Reader, error) {
	if len(cfg.Brokers) == 0 || hub == nil || logger == nil {
		return nil, errors.New("realtime reader: invalid configuration")
	}
	topics := []string{
		platformkafka.FullTopic(cfg.TopicPrefix, platformkafka.CatalogTopic),
		platformkafka.FullTopic(cfg.TopicPrefix, platformkafka.DeliveriesTopic),
		platformkafka.FullTopic(cfg.TopicPrefix, platformkafka.StockTopic),
		platformkafka.FullTopic(cfg.TopicPrefix, platformkafka.OrdersTopic),
		platformkafka.FullTopic(cfg.TopicPrefix, platformkafka.ExpiryTopic),
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID("goden-api-realtime"),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, fmt.Errorf("realtime reader: create Kafka client: %w", err)
	}
	return &Reader{client: client, hub: hub, logger: logger, topics: topics}, nil
}

func (r *Reader) Close() { r.client.Close() }

// Run повторяет чтение после временных ошибок Kafka и завершается только вместе
// с контекстом API. Поэтому недоступность broker не влияет на готовность REST.
func (r *Reader) Run(ctx context.Context) error {
	provisioned := false
	for {
		if !provisioned {
			if err := platformkafka.EnsureTopics(ctx, r.client, r.topics); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				r.logger.ErrorContext(ctx, "realtime Kafka topic provisioning failed", "error", err)
				if !waitReader(ctx, readerRetryDelay) {
					return nil
				}
				continue
			}
			provisioned = true
		}
		fetches := r.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		fetchErrors := fetches.Errors()
		if len(fetchErrors) > 0 {
			provisioned = false
		}
		for _, fetchErr := range fetchErrors {
			r.logger.ErrorContext(ctx, "realtime Kafka read failed",
				"topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err)
		}
		for _, record := range fetches.Records() {
			if _, err := r.Process(record.Value); err != nil {
				r.logger.ErrorContext(ctx, "realtime Kafka event skipped",
					"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			}
		}
		if len(fetchErrors) > 0 && len(fetches.Records()) == 0 && !waitReader(ctx, readerRetryDelay) {
			return nil
		}
	}
}

// Process декодирует одну запись и публикует сигнал, если событие меняет одно
// из представлений, доступных клиенту через REST.
func (r *Reader) Process(raw []byte) (bool, error) {
	var event platformkafka.Envelope
	if err := json.Unmarshal(raw, &event); err != nil {
		return false, fmt.Errorf("decode envelope: %w", err)
	}
	if event.EnvelopeVersion != platformkafka.EnvelopeVersion || event.EventID == uuid.Nil || event.EventType == "" {
		return false, fmt.Errorf("unsupported envelope_version %d or empty event_id/event_type", event.EnvelopeVersion)
	}
	update, ok := FromEnvelope(event)
	if !ok {
		return false, nil
	}
	r.hub.Publish(update)
	return true, nil
}

func waitReader(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
