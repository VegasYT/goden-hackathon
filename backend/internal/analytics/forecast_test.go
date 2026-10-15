package analytics

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
)

func TestForecastUsesConsumptionRate(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	at, cause, ok := productForecast(now, 10*time.Hour, 24*time.Hour, 10, 50,
		[]inventory.Batch{{Quantity: 100, ExpiresAt: now.Add(48 * time.Hour)}})
	require.True(t, ok)
	require.Equal(t, now.Add(18*time.Hour), at)
	require.Equal(t, CauseConsumption, cause)
}

func TestForecastAccountsForExpiryAfterFEFOConsumption(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	at, cause, ok := productForecast(now, 10*time.Hour, 24*time.Hour, 10, 10, []inventory.Batch{
		{Quantity: 80, ExpiresAt: now.Add(5 * time.Hour)},
		{Quantity: 20, ExpiresAt: now.Add(100 * time.Hour)},
	})
	require.True(t, ok)
	require.Equal(t, now.Add(15*time.Hour), at)
	// после истечения первой партии на порцию ещё хватает, дальше остаток израсходуют
	require.Equal(t, CauseConsumption, cause)
}

// Расход идёт, но раньше, чем его израсходуют, истекает партия, без которой на порцию не хватит.
func TestForecastCauseIsExpiryWhenBatchLeavesLessThanPortion(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	at, cause, ok := productForecast(now, 10*time.Hour, 24*time.Hour, 10, 10, []inventory.Batch{
		{Quantity: 95, ExpiresAt: now.Add(2 * time.Hour)},
		{Quantity: 5, ExpiresAt: now.Add(100 * time.Hour)},
	})
	require.True(t, ok)
	require.Equal(t, now.Add(2*time.Hour), at)
	require.Equal(t, CauseExpiry, cause)
}

func TestForecastChoosesEarliestIngredientAndSkipsStoppedDish(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	creamID, baconID := uuid.New(), uuid.New()
	cream := catalog.Product{ID: creamID, Name: "Сливки", Unit: catalog.UnitMilliliter}
	bacon := catalog.Product{ID: baconID, Name: "Бекон", Unit: catalog.UnitGram}
	dishes := []catalog.Dish{
		{ID: uuid.New(), Name: "Паста", Ingredients: []catalog.Ingredient{
			{ProductID: creamID, Quantity: 10}, {ProductID: baconID, Quantity: 10},
		}},
		{ID: uuid.New(), Name: "Недоступное блюдо", Ingredients: []catalog.Ingredient{{ProductID: baconID, Quantity: 40}}},
	}
	forecasts := buildForecasts(now, 10*time.Hour, 24*time.Hour, dishes,
		map[uuid.UUID]catalog.Product{creamID: cream, baconID: bacon},
		map[uuid.UUID][]inventory.Batch{
			creamID: {{Quantity: 100, ExpiresAt: now.Add(48 * time.Hour)}},
			baconID: {{Quantity: 30, ExpiresAt: now.Add(48 * time.Hour)}},
		},
		map[uuid.UUID]int64{creamID: 50, baconID: 20})

	require.Len(t, forecasts, 1)
	require.Equal(t, "Паста", forecasts[0].Dish.Name)
	require.Equal(t, bacon, forecasts[0].Product)
	require.Equal(t, now.Add(10*time.Hour), forecasts[0].ExpectedAt)
	// объяснение относится к ограничивающему продукту: 20 за 10 часов - 2 в час
	require.Equal(t, int64(30), forecasts[0].Available)
	require.Equal(t, int64(10), forecasts[0].Required)
	require.InDelta(t, 2.0, forecasts[0].ConsumptionPerHour, 1e-9)
	require.Equal(t, CauseConsumption, forecasts[0].Cause)
}

func TestForecastRequiresConsumptionAndHonorsHorizon(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	batches := []inventory.Batch{{Quantity: 100, ExpiresAt: now.Add(48 * time.Hour)}}
	_, _, ok := productForecast(now, 10*time.Hour, 24*time.Hour, 10, 0, batches)
	require.False(t, ok)
	_, _, ok = productForecast(now, 10*time.Hour, 5*time.Hour, 10, 50, batches)
	require.False(t, ok)
}

// Расход делится на время, за которое есть данные, а не на всё окно: первый расход был
// 30 минут назад, израсходовано 3000, значит темп 6000 в час, а не 3000 / 24 = 125.
func TestObservedPeriodFollowsTheLengthOfHistory(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	window := 24 * time.Hour

	tests := []struct {
		name  string
		first time.Time
		want  time.Duration
	}{
		{"short history", now.Add(-30 * time.Minute), 30 * time.Minute},
		{"history shorter than the minimum", now.Add(-time.Minute), minObservedPeriod},
		{"no consumption at all", now, minObservedPeriod},
		{"history fills the window", now.Add(-24 * time.Hour), window},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, observedPeriod(now, tt.first, window))
		})
	}

	observed := observedPeriod(now, now.Add(-30*time.Minute), window)
	at, cause, ok := productForecast(now, observed, 168*time.Hour, 100, 3000,
		[]inventory.Batch{{Quantity: 20000, ExpiresAt: now.Add(400 * time.Hour)}})
	require.True(t, ok)
	// (20000 - 100) / 6000 часа; при делении на всё окно вышло бы 159 часов
	require.WithinDuration(t, now.Add(199*time.Minute), at, time.Second)
	require.Equal(t, CauseConsumption, cause)
}

// Без расхода остаток убывает только по срокам годности: блюдо уйдёт в стоп-лист,
// когда истечёт партия, без которой на порцию не хватает.
func TestForecastWithoutConsumptionFollowsExpiry(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)

	at, cause, ok := productForecast(now, time.Hour, 24*time.Hour, 10, 0, []inventory.Batch{
		{Quantity: 95, ExpiresAt: now.Add(2 * time.Hour)},
		{Quantity: 5, ExpiresAt: now.Add(100 * time.Hour)},
	})
	require.True(t, ok)
	require.Equal(t, now.Add(2*time.Hour), at)
	require.Equal(t, CauseExpiry, cause)

	// после истечения первой партии на порцию ещё хватает
	_, _, ok = productForecast(now, time.Hour, 24*time.Hour, 10, 0, []inventory.Batch{
		{Quantity: 80, ExpiresAt: now.Add(2 * time.Hour)},
		{Quantity: 20, ExpiresAt: now.Add(100 * time.Hour)},
	})
	require.False(t, ok)
}
