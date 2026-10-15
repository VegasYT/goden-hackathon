// Package estest содержит помощники для тестов, работающих с событиями.
package estest

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

// Append записывает готовые события в конец потока отдельной транзакцией.
// Нужен тестам проекций: они наполняют Event Store напрямую, без команд.
func Append(t testing.TB, pool *pgxpool.Pool, streamID string, payloads ...eventstore.Payload) {
	t.Helper()
	AppendAt(t, pool, streamID, time.Now(), payloads...)
}

// AppendAt делает то же, что Append, но с заданным временем события.
// Нужен, когда проекция опирается на время из конверта события: у StockConsumed
// и StockWrittenOff своего времени в содержимом нет.
func AppendAt(t testing.TB, pool *pgxpool.Pool, streamID string, occurredAt time.Time, payloads ...eventstore.Payload) {
	t.Helper()
	ctx := context.Background()

	events := make([]eventstore.NewEvent, len(payloads))
	for i, p := range payloads {
		raw, err := eventstore.Encode(p)
		require.NoError(t, err)
		events[i] = eventstore.NewEvent{
			ID:            uuid.Must(uuid.NewV7()),
			Type:          p.EventType(),
			SchemaVersion: p.SchemaVersion(),
			Payload:       raw,
			OccurredAt:    occurredAt,
		}
	}

	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		version, err := eventstore.StreamVersion(ctx, tx, streamID)
		if err != nil {
			return err
		}
		_, err = eventstore.Append(ctx, tx, streamID, version, events...)
		return err
	})
	require.NoError(t, err)
}

// Golden сверяет JSON события с эталоном testdata/<тип>.json и проверяет,
// что событие восстанавливается из JSON без потерь.
//
// Эталон - контракт между записью и чтением. Если формат изменён намеренно,
// перезапиши эталоны: UPDATE_GOLDEN=1 go test ./internal/...
func Golden(t *testing.T, sample eventstore.Payload) {
	t.Helper()

	raw, err := eventstore.Encode(sample)
	require.NoError(t, err)
	var pretty bytes.Buffer
	require.NoError(t, json.Indent(&pretty, raw, "", "  "))

	path := filepath.Join("testdata", sample.EventType()+".json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, append(pretty.Bytes(), '\n'), 0o644))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "no golden file: run with UPDATE_GOLDEN=1")
	// git на Windows может заменить переводы строк при checkout
	wantText := strings.TrimSpace(strings.ReplaceAll(string(want), "\r\n", "\n"))
	require.Equal(t, wantText, pretty.String(), "JSON of %s changed: the format is a contract", sample.EventType())

	decoded, err := eventstore.Decode(sample.EventType(), sample.SchemaVersion(), raw)
	require.NoError(t, err)
	require.Equal(t, sample, decoded)
}
