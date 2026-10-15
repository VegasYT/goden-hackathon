package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// Config - общие настройки Kafka для backend-процессов.
type Config struct {
	Brokers     []string `env:"KAFKA_BROKERS" envDefault:"localhost:9092" envSeparator:","`
	TopicPrefix string   `env:"KAFKA_TOPIC_PREFIX" envDefault:"goden"`
}

// produceTimeout - срок на подтверждение записи. Без него отправка при потерянном соединении
// ждёт без конца: publisher не возвращает ошибку, и сбой Kafka не виден в метриках.
const produceTimeout = 10 * time.Second

// Producer подтверждает сообщение только после broker ack.
type Producer interface {
	Produce(context.Context, Message) error
}

// Client оборачивает franz-go, оставляя остальному пакету маленький интерфейс.
type Client struct {
	inner *kgo.Client
	mu    sync.Mutex
	known map[string]struct{}
}

func NewProducer(cfg Config, clientID string) (*Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: at least one broker is required")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(produceTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: create producer: %w", err)
	}
	return &Client{inner: client, known: make(map[string]struct{})}, nil
}

func (c *Client) Produce(ctx context.Context, message Message) error {
	// срок на весь вызов: клиент может ждать дольше RecordDeliveryTimeout, пока восстанавливает соединение
	ctx, cancel := context.WithTimeout(ctx, 2*produceTimeout)
	defer cancel()
	if err := c.ensureTopic(ctx, message.Topic); err != nil {
		return err
	}
	result := c.inner.ProduceSync(ctx, &kgo.Record{Topic: message.Topic, Key: message.Key, Value: message.Value})
	if err := result.FirstErr(); err != nil {
		c.mu.Lock()
		delete(c.known, message.Topic)
		c.mu.Unlock()
		return fmt.Errorf("kafka: produce to %s: %w", message.Topic, err)
	}
	return nil
}

func (c *Client) Close() { c.inner.Close() }

// EnsureTopics создаёт уже полные имена топиков через существующий Kafka-клиент.
// Функция нужна процессам, которые читают топики до первой публикации.
func EnsureTopics(ctx context.Context, client *kgo.Client, topics []string) error {
	return ensureTopics(ctx, client, topics)
}

func (c *Client) ensureTopic(ctx context.Context, topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.known[topic]; ok {
		return nil
	}
	if err := ensureTopics(ctx, c.inner, []string{topic}); err != nil {
		return fmt.Errorf("kafka: ensure topic %s: %w", topic, err)
	}
	c.known[topic] = struct{}{}
	return nil
}

// ensureTopics идемпотентно создаёт топики с настройками broker по умолчанию.
// Явный provisioning не зависит от allow.auto.create.topics в окружении.
func ensureTopics(ctx context.Context, client *kgo.Client, topics []string) error {
	request := kmsg.NewPtrCreateTopicsRequest()
	request.Topics = make([]kmsg.CreateTopicsRequestTopic, 0, len(topics))
	for _, topic := range topics {
		request.Topics = append(request.Topics, kmsg.CreateTopicsRequestTopic{
			Topic: topic, NumPartitions: -1, ReplicationFactor: -1,
		})
	}
	raw, err := client.Request(ctx, request)
	if err != nil {
		return fmt.Errorf("create topics request: %w", err)
	}
	response, ok := raw.(*kmsg.CreateTopicsResponse)
	if !ok {
		return fmt.Errorf("unexpected create topics response %T", raw)
	}
	for _, topic := range response.Topics {
		cause := kerr.ErrorForCode(topic.ErrorCode)
		if cause != nil && !errors.Is(cause, kerr.TopicAlreadyExists) {
			return fmt.Errorf("create topic %s: %w", topic.Topic, cause)
		}
	}
	return nil
}
