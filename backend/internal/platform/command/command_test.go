package command_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

// noted - событие для тестов пакета.
type noted struct {
	Note string `json:"note"`
}

func (noted) EventType() string  { return "Noted" }
func (noted) SchemaVersion() int { return 1 }

func init() { eventstore.Register[noted]() }

var (
	startedAt = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	manager   = eventstore.Actor{Type: eventstore.ActorUser, ID: "user-1"}
	quiet     = slog.New(slog.DiscardHandler)
)

// Некорректное описание отклоняется до обращения к базе: пул здесь nil.
func TestRunRejectsInvalidSpec(t *testing.T) {
	runner := command.NewRunner(nil, clock.NewFake(startedAt), quiet)

	tests := []struct {
		name string
		spec command.Spec
	}{
		{"no name", command.Spec{Actor: manager}},
		{"no actor", command.Spec{Name: "Test"}},
		{"unknown actor type", command.Spec{Name: "Test", Actor: eventstore.Actor{Type: "robot", ID: "x"}}},
		{"empty stream in locks", command.Spec{Name: "Test", Actor: manager, Locks: []string{"stock-1", ""}}},
		{"request is not serializable", command.Spec{
			Name: "Test", Actor: manager, IdempotencyKey: "k", Request: func() {},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			_, err := command.Run(context.Background(), runner, tt.spec,
				func(context.Context, *command.Tx) (struct{}, error) {
					called = true
					return struct{}{}, nil
				})

			require.ErrorIs(t, err, command.ErrInvalidSpec)
			require.False(t, called)
		})
	}
}
