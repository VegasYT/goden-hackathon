package eventstore_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

var occurredAt = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func newEvent(eventType string) eventstore.NewEvent {
	return eventstore.NewEvent{
		ID:            uuid.Must(uuid.NewV7()),
		Type:          eventType,
		SchemaVersion: 1,
		Payload:       json.RawMessage(`{"n":1}`),
		OccurredAt:    occurredAt,
	}
}

// Некорректный вызов отклоняется до обращения к базе: транзакция здесь nil.
func TestAppendRejectsInvalidInput(t *testing.T) {
	noType := newEvent("")
	noID := newEvent("Tested")
	noID.ID = uuid.Nil
	badPayload := newEvent("Tested")
	badPayload.Payload = json.RawMessage(`{`)

	tests := []struct {
		name     string
		streamID string
		events   []eventstore.NewEvent
	}{
		{"empty stream id", "", []eventstore.NewEvent{newEvent("Tested")}},
		{"no events", "s-1", nil},
		{"event without type", "s-1", []eventstore.NewEvent{noType}},
		{"event without id", "s-1", []eventstore.NewEvent{noID}},
		{"invalid payload", "s-1", []eventstore.NewEvent{badPayload}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := eventstore.Append(context.Background(), nil, tt.streamID, 0, tt.events...)
			require.ErrorIs(t, err, eventstore.ErrInvalidInput)
		})
	}
}

func TestReadAllRejectsNonPositiveLimit(t *testing.T) {
	_, err := eventstore.ReadAll(context.Background(), nil, eventstore.Checkpoint{}, 0)
	require.ErrorIs(t, err, eventstore.ErrInvalidInput)
}
