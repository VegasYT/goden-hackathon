package inventory

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPlanConsumption(t *testing.T) {
	day := func(n int) time.Time { return now.Add(time.Duration(n) * 24 * time.Hour) }
	b := func(quantity int64, expiresAt time.Time) Batch {
		return Batch{ID: uuid.Must(uuid.NewV7()), Quantity: quantity, ExpiresAt: expiresAt}
	}
	// партии в порядке FEFO, как их отдаёт запрос
	expired, first, second, third := b(100, day(-1)), b(100, day(2)), b(100, day(5)), b(100, day(10))
	atExpiry, empty := b(100, now), b(0, day(1))

	tests := []struct {
		name          string
		batches       []Batch
		need          int64
		wantTaken     []BatchQuantity
		wantAvailable int64
	}{
		{"single batch", []Batch{first}, 30, []BatchQuantity{{first.ID, 30}}, 100},
		{
			"three batches in expiry order",
			[]Batch{first, second, third},
			250,
			[]BatchQuantity{{first.ID, 100}, {second.ID, 100}, {third.ID, 50}},
			300,
		},
		{"nearest batch is enough", []Batch{first, second}, 30, []BatchQuantity{{first.ID, 30}}, 200},
		{
			"exactly the whole stock",
			[]Batch{first, second},
			200,
			[]BatchQuantity{{first.ID, 100}, {second.ID, 100}},
			200,
		},
		{"shortage takes nothing", []Batch{first, second}, 201, nil, 200},
		{
			"expired batch is skipped",
			[]Batch{expired, first, second},
			150,
			[]BatchQuantity{{first.ID, 100}, {second.ID, 50}},
			200,
		},
		{"expired batch does not count as available", []Batch{expired, first}, 150, nil, 100},
		{"batch expiring right now is expired", []Batch{atExpiry, first}, 150, nil, 100},
		{"empty batch is skipped", []Batch{empty, first}, 30, []BatchQuantity{{first.ID, 30}}, 100},
		{"no batches", nil, 1, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taken, available := planConsumption(tt.batches, tt.need, now)

			require.Equal(t, tt.wantTaken, taken)
			require.Equal(t, tt.wantAvailable, available)
		})
	}
}
