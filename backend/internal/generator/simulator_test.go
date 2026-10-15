package generator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccumulator(t *testing.T) {
	tests := []struct {
		name      string
		perMinute int32
		tick      time.Duration
		ticks     int
		want      []int // действий за каждый из первых тиков; nil - не проверяется
		wantTotal int
	}{
		{"12 per minute is one action every five ticks", 12, time.Second, 60, []int{0, 0, 0, 0, 1, 0, 0, 0, 0, 1}, 12},
		{"zero rate gives nothing", 0, time.Second, 600, nil, 0},
		{"fractions are carried over", 7, time.Second, 60, nil, 7},
		{"short ticks do not lose actions", 600, 20 * time.Millisecond, 3000, nil, 600},
		{"several actions per tick", 600, time.Second, 60, []int{10, 10}, 600},
		{"actions per tick are capped", 600, time.Minute, 1, []int{maxPerTick}, maxPerTick},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				a     accumulator
				total int
			)
			for i := range tt.ticks {
				n := a.due(tt.perMinute, tt.tick)
				if i < len(tt.want) {
					require.Equal(t, tt.want[i], n, "tick %d", i)
				}
				total += n
			}
			require.Equal(t, tt.wantTotal, total)
		})
	}
}
