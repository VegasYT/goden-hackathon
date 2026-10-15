package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	platformkafka "goden/backend/internal/platform/kafka"
)

func TestErrorDeliveryIsDeterministicAndOmitsOriginal(t *testing.T) {
	event := platformkafka.ErrorEnvelope{
		Consumer: "analytics", SourceTopic: "goden.stock.v1",
		EventID: uuid.MustParse("0198c0de-0000-7000-8000-000000000001"), EventType: "StockWrittenOff",
		Error: "unsupported\npayload", Original: []byte(`{"secret":"must-not-leak"}`),
		FailedAt: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)

	first, _, ok := errorDelivery(raw)
	require.True(t, ok)
	second, _, ok := errorDelivery(raw)
	require.True(t, ok)
	require.Equal(t, first.SourceKey, second.SourceKey)
	require.Contains(t, first.Body, "Обработчик: analytics")
	require.Contains(t, first.Body, "Ошибка: unsupported payload")
	require.NotContains(t, first.Body, "must-not-leak")
}

func TestErrorDeliveryRejectsMalformedWithoutCreatingAnotherError(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`not-json`),
		[]byte(`{"consumer":"","source_topic":"goden.stock.v1","error":"bad"}`),
	} {
		_, _, ok := errorDelivery(raw)
		require.False(t, ok)
	}
}

// Повторная запись об одном и том же событии отличается временем сбоя: ключ уведомления
// строится по обработчику и событию, поэтому второе уведомление не создаётся.
func TestErrorDeliveryKeyIgnoresFailureTime(t *testing.T) {
	event := platformkafka.ErrorEnvelope{
		Consumer: "analytics", SourceTopic: "goden.stock.v1",
		EventID: uuid.MustParse("0198c0de-0000-7000-8000-000000000001"), EventType: "StockWrittenOff",
		Error: "unsupported payload", FailedAt: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
	}
	key := func(e platformkafka.ErrorEnvelope) string {
		raw, err := json.Marshal(e)
		require.NoError(t, err)
		delivery, _, ok := errorDelivery(raw)
		require.True(t, ok)
		return delivery.SourceKey
	}
	first := key(event)

	event.FailedAt = event.FailedAt.Add(time.Minute)
	require.Equal(t, first, key(event), "the same event of the same consumer")

	event.Consumer = "notifications-domain"
	require.NotEqual(t, first, key(event), "another consumer is another notification")

	// у записи без события ключ - по содержимому
	event.EventID = uuid.Nil
	withoutEvent := key(event)
	event.Error = "another failure"
	require.NotEqual(t, withoutEvent, key(event))
}
