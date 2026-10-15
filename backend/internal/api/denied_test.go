package api

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
)

// Отказ в доступе находят в журнале по операции и автору: и отказ по праву, и отказ правила
// про управляющих, который возвращает обработчик.
func TestDeniedRequestIsLoggedWithOperationAndActor(t *testing.T) {
	actor := eventstore.Actor{Type: eventstore.ActorUser, ID: "cook"}
	for name, cause := range map[string]error{
		"permission":  &forbiddenError{actor: actor, permission: "orders.accept"},
		"domain rule": fmt.Errorf("assign: %w", organization.ErrForbidden),
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			h := &Handler{logger: slog.New(slog.NewJSONHandler(&out, nil))}
			problem := h.NewError(context.Background(), &deniedError{operation: "PlaceOrder", actor: actor, err: cause})
			require.Equal(t, 403, problem.StatusCode)
			require.Contains(t, out.String(), `"operation":"PlaceOrder"`)
			require.Contains(t, out.String(), `"actor_id":"cook"`)
		})
	}
}
