package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/observability"
)

func TestLogContainsCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLogger(&buf, slog.LevelInfo).With("component", "test")
	id := uuid.MustParse("0198c0de-0000-7000-8000-000000000001")

	logger.InfoContext(observability.WithCorrelationID(context.Background(), id), "order accepted")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	require.Equal(t, id.String(), rec["correlation_id"])
	require.Equal(t, "order accepted", rec["msg"])
	require.Equal(t, "INFO", rec["level"])
	require.Equal(t, "test", rec["component"])
	require.Contains(t, rec, "time")
}

func TestLogLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLogger(&buf, slog.LevelWarn)

	logger.Info("skipped")
	logger.Error("kept")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 1)
	require.Contains(t, lines[0], `"msg":"kept"`)
}
