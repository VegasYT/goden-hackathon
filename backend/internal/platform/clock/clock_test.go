package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/clock"
)

func TestSystemReturnsUTC(t *testing.T) {
	require.Equal(t, time.UTC, clock.System{}.Now().Location())
}

func TestFakeAdvance(t *testing.T) {
	start := time.Date(2026, 10, 10, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	c := clock.NewFake(start)

	c.Advance(48 * time.Hour)

	require.True(t, c.Now().Equal(start.Add(48*time.Hour)))
	require.Equal(t, time.UTC, c.Now().Location())
	// время не идёт само
	require.Equal(t, c.Now(), c.Now())

	c.Set(start)
	require.True(t, c.Now().Equal(start))
}
