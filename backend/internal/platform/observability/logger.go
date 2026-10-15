// Package observability содержит логгер и передачу correlation ID через контекст.
package observability

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
)

type correlationKey struct{}

// WithCorrelationID помещает correlation ID в контекст запроса или обработки события.
func WithCorrelationID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}

func CorrelationID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(correlationKey{}).(uuid.UUID)
	return id, ok
}

// NewLogger пишет JSON по одной записи на строку. Correlation ID попадает в запись,
// только если лог пишется методами с контекстом (InfoContext и подобными).
func NewLogger(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(correlationHandler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

type correlationHandler struct{ slog.Handler }

func (h correlationHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := CorrelationID(ctx); ok {
		r.AddAttrs(slog.String("correlation_id", id.String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h correlationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return correlationHandler{h.Handler.WithAttrs(attrs)}
}

func (h correlationHandler) WithGroup(name string) slog.Handler {
	return correlationHandler{h.Handler.WithGroup(name)}
}
