package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	platformkafka "goden/backend/internal/platform/kafka"
)

func TestReaderProcessesSupportedEvent(t *testing.T) {
	hub := NewHub()
	reader := &Reader{hub: hub, logger: slog.New(slog.DiscardHandler)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := hub.Subscribe(ctx, scope)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	event := platformkafka.Envelope{
		EnvelopeVersion: platformkafka.EnvelopeVersion,
		EventID:         uuid.Must(uuid.NewV7()), EventType: "StockConsumed", OccurredAt: now,
		Payload: json.RawMessage(`{"point_id":"` + scope.PointID.String() + `"}`),
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)

	published, err := reader.Process(raw)
	require.NoError(t, err)
	require.True(t, published)
	require.Equal(t, Update{ID: event.EventID, OccurredAt: now, Categories: stockCategories, PointID: scope.PointID}, <-updates)
}

func TestReaderIgnoresEventWithoutInvalidation(t *testing.T) {
	reader := &Reader{hub: NewHub(), logger: slog.New(slog.DiscardHandler)}
	raw, err := json.Marshal(platformkafka.Envelope{
		EnvelopeVersion: platformkafka.EnvelopeVersion,
		EventID:         uuid.Must(uuid.NewV7()), EventType: "CompanyOwnerChanged", OccurredAt: time.Now(),
	})
	require.NoError(t, err)

	published, err := reader.Process(raw)
	require.NoError(t, err)
	require.False(t, published)
}

func TestReaderRejectsMalformedEnvelope(t *testing.T) {
	reader := &Reader{hub: NewHub(), logger: slog.New(slog.DiscardHandler)}

	_, err := reader.Process([]byte(`{"envelope_version":1}`))
	require.ErrorContains(t, err, "empty event_id/event_type")
	_, err = reader.Process([]byte(`{`))
	require.ErrorContains(t, err, "decode envelope")
}

func TestNewReaderDoesNotRequireAvailableBroker(t *testing.T) {
	reader, err := NewReader(platformkafka.Config{
		Brokers: []string{"127.0.0.1:1"}, TopicPrefix: "test",
	}, NewHub(), slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	reader.Close()
}
