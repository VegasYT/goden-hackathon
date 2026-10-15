package eventstore

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ErrUnknownEventType - в реестре нет события с таким типом и версией схемы.
var ErrUnknownEventType = errors.New("eventstore: unknown event type")

// Payload - содержимое доменного события. Реализуется типами-значениями в events.go модулей.
type Payload interface {
	EventType() string
	SchemaVersion() int
}

// TypeInfo - ключ реестра: тип события и версия его схемы.
type TypeInfo struct {
	Type          string
	SchemaVersion int
}

// реестр заполняется только из init() модулей, поэтому без блокировок
var registry = map[TypeInfo]func(json.RawMessage) (Payload, error){}

// Register добавляет тип события в реестр. Вызывается из init() модуля-владельца.
func Register[T Payload]() {
	var zero T
	key := TypeInfo{Type: zero.EventType(), SchemaVersion: zero.SchemaVersion()}
	if _, exists := registry[key]; exists {
		panic(fmt.Sprintf("eventstore: event %s v%d registered twice", key.Type, key.SchemaVersion))
	}
	registry[key] = func(raw json.RawMessage) (Payload, error) {
		var v T
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

// Encode сериализует содержимое события в JSON для записи в Event Store.
func Encode(p Payload) (json.RawMessage, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("eventstore: encode %s: %w", p.EventType(), err)
	}
	return raw, nil
}

// Decode восстанавливает содержимое события по типу и версии схемы.
func Decode(eventType string, schemaVersion int, raw json.RawMessage) (Payload, error) {
	decode, ok := registry[TypeInfo{Type: eventType, SchemaVersion: schemaVersion}]
	if !ok {
		return nil, fmt.Errorf("%s v%d: %w", eventType, schemaVersion, ErrUnknownEventType)
	}
	p, err := decode(raw)
	if err != nil {
		return nil, fmt.Errorf("eventstore: decode %s v%d: %w", eventType, schemaVersion, err)
	}
	return p, nil
}

// RegisteredTypes возвращает зарегистрированные типы событий в алфавитном порядке.
func RegisteredTypes() []TypeInfo {
	types := make([]TypeInfo, 0, len(registry))
	for key := range registry {
		types = append(types, key)
	}
	slices.SortFunc(types, func(a, b TypeInfo) int {
		return cmp.Or(strings.Compare(a.Type, b.Type), cmp.Compare(a.SchemaVersion, b.SchemaVersion))
	})
	return types
}

// Time - момент времени в событии. В JSON всегда записывается в UTC.
type Time struct{ time.Time }

// At приводит момент времени к UTC.
func At(t time.Time) Time { return Time{t.UTC()} }

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.UTC().Format(time.RFC3339Nano))
}

func (t *Time) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	t.Time = parsed.UTC()
	return nil
}

// ActorType - кто выполнил действие: пользователь или сервисная учётная запись.
type ActorType string

const (
	ActorUser    ActorType = "user"
	ActorService ActorType = "service"
)

func (a *ActorType) UnmarshalText(text []byte) error {
	return ParseEnum(a, text, "actor type", ActorUser, ActorService)
}

// Actor - автор действия, породившего событие.
type Actor struct {
	Type ActorType `json:"type"`
	ID   string    `json:"id"`
}

// ParseEnum разбирает значение закрытого перечисления и отклоняет неизвестное.
// Используется в UnmarshalText перечислений из событий.
func ParseEnum[T ~string](dst *T, text []byte, name string, allowed ...T) error {
	v := T(text)
	if !slices.Contains(allowed, v) {
		return fmt.Errorf("unknown %s %q", name, text)
	}
	*dst = v
	return nil
}
