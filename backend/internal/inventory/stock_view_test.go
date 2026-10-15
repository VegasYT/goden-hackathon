package inventory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
)

func TestStockLevelStatus(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	in := func(d time.Duration) *time.Time {
		at := now.Add(d)
		return &at
	}
	tests := []struct {
		name  string
		level StockLevel
		want  []StockStatus
	}{
		{"good batch, a week left", StockLevel{Available: 100, NearestExpiresAt: in(7 * 24 * time.Hour)}, []StockStatus{StockInStock}},
		{"expires in 20 hours", StockLevel{Available: 100, NearestExpiresAt: in(20 * time.Hour)}, []StockStatus{StockInStock, StockExpiring}},
		{"exactly a day left", StockLevel{Available: 100, NearestExpiresAt: in(24 * time.Hour)}, []StockStatus{StockInStock, StockExpiring}},
		{"a second over a day", StockLevel{Available: 100, NearestExpiresAt: in(24*time.Hour + time.Second)}, []StockStatus{StockInStock}},
		{
			"good and expired batches",
			StockLevel{Available: 40, Expired: 25, NearestExpiresAt: in(72 * time.Hour)},
			[]StockStatus{StockInStock, StockExpired},
		},
		{"only an expired batch", StockLevel{Expired: 25}, []StockStatus{StockExpired, StockOutOfStock}},
		{"no batches", StockLevel{}, []StockStatus{StockOutOfStock}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []StockStatus
			for _, status := range StockStatuses {
				if tt.level.Is(status, now) {
					got = append(got, status)
				}
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSortStock(t *testing.T) {
	at := func(day int) *time.Time {
		date := time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC)
		return &date
	}
	// порядок названий - как его отдаёт каталог
	levels := func() []StockLevel {
		return []StockLevel{
			{Product: catalog.Product{Name: "А"}, Available: 50, NearestExpiresAt: at(20)},
			{Product: catalog.Product{Name: "Б"}},
			{Product: catalog.Product{Name: "В"}, Available: 50, NearestExpiresAt: at(12)},
			{Product: catalog.Product{Name: "Г"}, Available: 10, NearestExpiresAt: at(15)},
		}
	}
	tests := []struct {
		name       string
		by         StockSort
		descending bool
		want       []string
	}{
		{"name", StockByName, false, []string{"А", "Б", "В", "Г"}},
		{"name, descending", StockByName, true, []string{"Г", "В", "Б", "А"}},
		{"available, equal values stay by name", StockByAvailable, false, []string{"Б", "Г", "А", "В"}},
		{"available, descending", StockByAvailable, true, []string{"А", "В", "Г", "Б"}},
		{"nearest expiry, without a date last", StockByNearestExpiry, false, []string{"В", "Г", "А", "Б"}},
		{"nearest expiry descending, without a date still last", StockByNearestExpiry, true, []string{"А", "Г", "В", "Б"}},
		{"unknown field is the name order", "", false, []string{"А", "Б", "В", "Г"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted := levels()
			sortStock(sorted, tt.by, tt.descending)
			names := make([]string, len(sorted))
			for i, level := range sorted {
				names[i] = level.Product.Name
			}
			require.Equal(t, tt.want, names)
		})
	}
}
