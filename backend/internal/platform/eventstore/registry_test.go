package eventstore_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

type sampleEvent struct {
	Note string          `json:"note"`
	At   eventstore.Time `json:"at"`
}

func (sampleEvent) EventType() string  { return "SampleEvent" }
func (sampleEvent) SchemaVersion() int { return 1 }

func init() { eventstore.Register[sampleEvent]() }

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := sampleEvent{Note: "hello", At: eventstore.At(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))}

	raw, err := eventstore.Encode(in)
	require.NoError(t, err)
	out, err := eventstore.Decode("SampleEvent", 1, raw)

	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestDecodeUnknown(t *testing.T) {
	tests := []struct {
		name          string
		eventType     string
		schemaVersion int
	}{
		{"unknown type", "NoSuchEvent", 1},
		{"unknown schema version", "SampleEvent", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := eventstore.Decode(tt.eventType, tt.schemaVersion, json.RawMessage(`{}`))

			require.ErrorIs(t, err, eventstore.ErrUnknownEventType)
			require.ErrorContains(t, err, tt.eventType)
		})
	}
}

func TestRegisterTwicePanics(t *testing.T) {
	require.Panics(t, func() { eventstore.Register[sampleEvent]() })
}

func TestTimeIsWrittenInUTC(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*3600)

	raw, err := json.Marshal(eventstore.Time{Time: time.Date(2026, 10, 10, 12, 0, 0, 0, moscow)})

	require.NoError(t, err)
	require.JSONEq(t, `"2026-10-10T09:00:00Z"`, string(raw))
}

func TestActorTypeIsClosedSet(t *testing.T) {
	var actor eventstore.Actor

	require.NoError(t, json.Unmarshal([]byte(`{"type":"service","id":"generator"}`), &actor))
	require.Equal(t, eventstore.Actor{Type: eventstore.ActorService, ID: "generator"}, actor)

	err := json.Unmarshal([]byte(`{"type":"robot","id":"x"}`), &actor)
	require.ErrorContains(t, err, "robot")
}
