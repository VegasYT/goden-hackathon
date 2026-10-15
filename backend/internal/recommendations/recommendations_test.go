package recommendations

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/stoplist"
)

var now = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)

const horizon = 72 * time.Hour

func product(name string) catalog.Product {
	return catalog.Product{ID: uuid.Must(uuid.NewV7()), Name: name, Unit: catalog.UnitGram}
}

// dish - доступное блюдо: portions порций, ingredients - пары "продукт, количество на порцию".
func dish(name string, portions int64, ingredients ...catalog.Ingredient) stoplist.Availability {
	return stoplist.Availability{
		Dish: catalog.Dish{ID: uuid.Must(uuid.NewV7()), Name: name, Ingredients: ingredients}, Portions: portions,
	}
}

func per(p catalog.Product, quantity int64) catalog.Ingredient {
	return catalog.Ingredient{ProductID: p.ID, Quantity: quantity}
}

// batch - партия продукта, истекающая через hours часов.
func batch(p catalog.Product, quantity int64, hours float64) inventory.ExpiringBatch {
	expiresAt := now.Add(time.Duration(hours * float64(time.Hour)))
	return inventory.ExpiringBatch{
		Product: p, Expired: !expiresAt.After(now),
		Batch: inventory.Batch{ID: uuid.Must(uuid.NewV7()), ProductID: p.ID, Quantity: quantity, ExpiresAt: expiresAt},
	}
}

func names(offers []DishOffer) []string {
	out := make([]string, len(offers))
	for i, o := range offers {
		out[i] = o.Dish.Name
	}
	return out
}

func TestHotOffersRanking(t *testing.T) {
	cream, bacon, eggs, salt := product("Сливки"), product("Бекон"), product("Яйцо"), product("Соль")

	t.Run("the closer the expiry, the higher the dish", func(t *testing.T) {
		offers := hotOffers(
			[]stoplist.Availability{dish("Поздний", 10, per(bacon, 100)), dish("Срочный", 10, per(cream, 100))},
			[]inventory.ExpiringBatch{batch(cream, 500, 10), batch(bacon, 500, 60)}, now, horizon)
		require.Equal(t, []string{"Срочный", "Поздний"}, names(offers))
		require.Greater(t, offers[0].Priority, offers[1].Priority)
	})
	t.Run("priority grows as time passes", func(t *testing.T) {
		menu := []stoplist.Availability{dish("Соус", 10, per(cream, 100))}
		expiring := []inventory.ExpiringBatch{batch(cream, 500, 60)}
		previous := -1
		for _, passed := range []time.Duration{0, 20 * time.Hour, 40 * time.Hour, 59 * time.Hour} {
			offers := hotOffers(menu, expiring, now.Add(passed), horizon)
			require.Len(t, offers, 1)
			require.Greater(t, offers[0].Priority, previous, "after %s", passed)
			previous = offers[0].Priority
		}
	})
	t.Run("a dish that can use up more of the risk is higher", func(t *testing.T) {
		offers := hotOffers(
			[]stoplist.Availability{dish("Щепотка", 10, per(cream, 5)), dish("Основа", 10, per(cream, 100))},
			[]inventory.ExpiringBatch{batch(cream, 1000, 10)}, now, horizon)
		require.Equal(t, []string{"Основа", "Щепотка"}, names(offers))
		require.InDelta(t, 1.0, offers[0].Reasons[0].Coverage, 1e-9, "10 portions of 100 use up all 1000")
		require.InDelta(t, 0.05, offers[1].Reasons[0].Coverage, 1e-9, "10 portions of 5 use up 50 of 1000")
	})
	t.Run("the only consumer of a product is higher", func(t *testing.T) {
		offers := hotOffers(
			[]stoplist.Availability{
				dish("Один из трёх A", 10, per(bacon, 100)), dish("Один из трёх B", 10, per(bacon, 100)),
				dish("Один из трёх C", 10, per(bacon, 100)), dish("Единственный", 10, per(cream, 100)),
			},
			[]inventory.ExpiringBatch{batch(cream, 500, 10), batch(bacon, 500, 10)}, now, horizon)
		require.Equal(t, "Единственный", offers[0].Dish.Name)
		require.InDelta(t, 1.0, offers[0].Reasons[0].Exclusivity, 1e-9)
		require.InDelta(t, 2.0/3, offers[1].Reasons[0].Exclusivity, 1e-9)
		require.Equal(t, 3, offers[1].Reasons[0].Dishes)
	})
	t.Run("two products at risk beat one", func(t *testing.T) {
		offers := hotOffers(
			[]stoplist.Availability{dish("Один продукт", 10, per(cream, 100)), dish("Два продукта", 10, per(cream, 100), per(eggs, 1))},
			[]inventory.ExpiringBatch{batch(cream, 500, 10), batch(eggs, 10, 10)}, now, horizon)
		require.Equal(t, []string{"Два продукта", "Один продукт"}, names(offers))
		require.Len(t, offers[0].Reasons, 2)
	})
	t.Run("risk smaller than a portion counts less", func(t *testing.T) {
		offers := hotOffers(
			[]stoplist.Availability{dish("Соус", 10, per(cream, 100))},
			[]inventory.ExpiringBatch{batch(cream, 20, 10)}, now, horizon)
		require.InDelta(t, 0.2, offers[0].Reasons[0].Significance, 1e-9)
	})
	t.Run("what is not recommended", func(t *testing.T) {
		stopped := stoplist.Availability{
			Dish:   catalog.Dish{ID: uuid.Must(uuid.NewV7()), Name: "В стоп-листе", Ingredients: []catalog.Ingredient{per(cream, 100), per(salt, 1)}},
			Reason: stoplist.InsufficientStock,
		}
		offers := hotOffers(
			[]stoplist.Availability{stopped, dish("Без риска", 10, per(bacon, 100)), dish("Просрочено", 10, per(eggs, 1))},
			[]inventory.ExpiringBatch{
				batch(cream, 500, 10),  // блюдо с ним в стоп-листе
				batch(bacon, 500, 100), // за горизонтом
				batch(eggs, 10, -5),    // уже просрочено
				batch(salt, 0, 10),     // пустая партия
			}, now, horizon)
		require.Empty(t, offers)
		require.Empty(t, hotOffers(nil, nil, now, horizon))
	})
}

func TestHotOfferExplanation(t *testing.T) {
	cream := product("Сливки")
	late, early := batch(cream, 300, 40), batch(cream, 200, 10)

	offers := hotOffers([]stoplist.Availability{dish("Соус", 8, per(cream, 100))},
		[]inventory.ExpiringBatch{late, early}, now, horizon)
	require.Len(t, offers, 1)
	offer := offers[0]
	require.EqualValues(t, 8, offer.Portions)
	require.EqualValues(t, 5, offer.SuggestedPortions, "500 at risk, 100 per portion")

	reason := offer.Reasons[0]
	require.Equal(t, cream, reason.Product)
	require.EqualValues(t, 500, reason.AtRisk, "both batches are at risk")
	require.Equal(t, early.Batch.ID, reason.NearestBatchID, "the batch an order consumes first")
	require.Equal(t, early.Batch.ExpiresAt, reason.NearestExpiresAt)
	require.EqualValues(t, 100, reason.PerPortion)
	require.Equal(t, 1, reason.Dishes)
	// срочность - по ближайшей партии, названной в объяснении: до её срока 10 часов из 72
	require.InDelta(t, 1-10.0/72, reason.Urgency, 1e-9)
	require.InDelta(t, 1, reason.Significance, 1e-9, "the nearest batch covers two portions")
	require.Equal(t, int(reason.Urgency*100+0.5), offer.Priority, "coverage, significance and exclusivity are 1")

	t.Run("no more portions than can be cooked", func(t *testing.T) {
		offers := hotOffers([]stoplist.Availability{dish("Соус", 3, per(cream, 100))},
			[]inventory.ExpiringBatch{early, late}, now, horizon)
		require.EqualValues(t, 3, offers[0].SuggestedPortions)
		require.InDelta(t, 0.6, offers[0].Reasons[0].Coverage, 1e-9)
	})
	t.Run("portions are rounded up to use everything", func(t *testing.T) {
		offers := hotOffers([]stoplist.Availability{dish("Соус", 10, per(cream, 100))},
			[]inventory.ExpiringBatch{batch(cream, 250, 10)}, now, horizon)
		require.EqualValues(t, 3, offers[0].SuggestedPortions)
	})
}

// Среднее по партиям гасило срочность: 200 единиц истекают через час, приходит 5000 со сроком
// через 70 часов - и приоритет падал с 99 до 6, хотя те же 200 единиц пропадут.
func TestLaterBatchDoesNotLowerThePriority(t *testing.T) {
	cream := product("Сливки")
	soon := batch(cream, 200, 1)
	sauce := func(portions int64) []stoplist.Availability {
		return []stoplist.Availability{dish("Соус", portions, per(cream, 100))}
	}

	before := hotOffers(sauce(2), []inventory.ExpiringBatch{soon}, now, horizon)[0]
	after := hotOffers(sauce(52), []inventory.ExpiringBatch{soon, batch(cream, 5000, 70)}, now, horizon)[0]

	require.Equal(t, 99, before.Priority)
	require.GreaterOrEqual(t, after.Priority, before.Priority, "a later batch must not lower the priority")
	reason := after.Reasons[0]
	require.Equal(t, soon.Batch.ID, reason.NearestBatchID)
	require.InDelta(t, 1-1.0/72, reason.Urgency, 1e-9, "urgency matches the batch named in the explanation")
	require.EqualValues(t, 5200, reason.AtRisk, "every batch within the horizon is at risk")
}

// Склад не меняется, время идёт: приоритет не падает, в том числе когда в горизонт
// входит ещё одна, более поздняя партия.
func TestPriorityGrowsWithTimeWithSeveralBatches(t *testing.T) {
	cream := product("Сливки")
	batches := []inventory.ExpiringBatch{batch(cream, 200, 3), batch(cream, 5000, 73)} // вторая пока за горизонтом
	menu := []stoplist.Availability{dish("Соус", 52, per(cream, 100))}

	previous := -1
	for minutes := 0; minutes <= 150; minutes += 30 {
		at := now.Add(time.Duration(minutes) * time.Minute)
		offers := hotOffers(menu, batches, at, horizon)
		require.Len(t, offers, 1)
		require.GreaterOrEqual(t, offers[0].Priority, previous, "after %d minutes", minutes)
		previous = offers[0].Priority
	}
	require.Greater(t, previous, 90)
}

// Ближайшей партии не хватает на порцию: спасать почти нечего, вклад продукта мал.
func TestNearestBatchSmallerThanAPortion(t *testing.T) {
	cream := product("Сливки")

	offers := hotOffers([]stoplist.Availability{dish("Соус", 30, per(cream, 100))},
		[]inventory.ExpiringBatch{batch(cream, 5, 1), batch(cream, 3000, 60)}, now, horizon)

	require.Len(t, offers, 1)
	reason := offers[0].Reasons[0]
	require.InDelta(t, 0.05, reason.Significance, 1e-9, "5 of 100 per portion")
	require.InDelta(t, 1-1.0/72, reason.Urgency, 1e-9)
	require.LessOrEqual(t, offers[0].Priority, 5)
}

// Две партии с одним сроком считаются ближайшими вместе.
func TestBatchesWithTheSameExpiryAreNearestTogether(t *testing.T) {
	cream := product("Сливки")

	offers := hotOffers([]stoplist.Availability{dish("Соус", 5, per(cream, 100))},
		[]inventory.ExpiringBatch{batch(cream, 60, 2), batch(cream, 60, 2)}, now, horizon)

	require.InDelta(t, 1, offers[0].Reasons[0].Significance, 1e-9, "120 together cover a portion")
}

func TestManualSettingsOverComputedOffers(t *testing.T) {
	cream, bacon := product("Сливки"), product("Бекон")
	pasta, soup := dish("Паста", 10, per(cream, 100)), dish("Суп", 10, per(bacon, 100))
	salad, steak := dish("Салат", 5), dish("Стейк", 0) // без риска; стейк в стоп-листе
	menu := []stoplist.Availability{pasta, soup, salad, steak}
	offers := hotOffers(menu, []inventory.ExpiringBatch{batch(cream, 500, 10), batch(bacon, 500, 60)}, now, horizon)
	require.Equal(t, []string{"Паста", "Суп"}, names(offers))

	set := func(d stoplist.Availability, mode HotOfferMode, minutes int) HotOfferSetting {
		return HotOfferSetting{DishID: d.Dish.ID, Mode: mode, Comment: "к вину", SetAt: now.Add(time.Duration(minutes) * time.Minute)}
	}
	got := withSettings(offers, menu, []HotOfferSetting{
		set(pasta, HotOfferHidden, 1), set(salad, HotOfferPinned, 2), set(steak, HotOfferPinned, 3), set(soup, HotOfferPinned, 4),
	})

	require.Equal(t, []string{"Салат", "Суп"}, names(got), "hidden is gone, unavailable pinned waits, pinned go in order of setting")
	require.Equal(t, int64(5), got[0].Portions)
	require.Empty(t, got[0].Reasons, "a pinned dish without risk has no reasons")
	require.NotEmpty(t, got[1].Reasons, "a pinned dish keeps the computed reasons")
	require.Equal(t, "к вину", got[1].Manual.Comment)

	require.Equal(t, names(offers), names(withSettings(offers, menu, nil)), "without settings the computation stays")
}

func TestSupplyAdvice(t *testing.T) {
	const day = 24 * time.Hour
	cream, bacon, eggs, salt, napkins := product("Сливки"), product("Бекон"), product("Яйцо"), product("Соль"), product("Салфетки")
	level := func(p catalog.Product, available, expired int64) inventory.StockLevel {
		return inventory.StockLevel{Product: p, Available: available, Expired: expired}
	}
	pasta := catalog.Dish{ID: uuid.Must(uuid.NewV7()), Name: "Паста", Ingredients: []catalog.Ingredient{per(cream, 100), per(salt, 2)}}
	omelette := catalog.Dish{ID: uuid.Must(uuid.NewV7()), Name: "Омлет", Ingredients: []catalog.Ingredient{per(eggs, 3), per(salt, 1)}}
	dishes := []catalog.Dish{pasta, omelette}

	advice := supplyAdvice(
		[]inventory.StockLevel{
			level(cream, 600, 0),  // расход 100 в час: хватит на 6 часов
			level(bacon, 5000, 0), // расход 100 в час: хватит на 50 часов
			level(eggs, 0, 40),    // всё просрочено, расходовалось
			level(salt, 0, 0),     // нет, не расходовалась, но нужна двум блюдам
			level(napkins, 0, 0),  // нет, никому не нужны
		},
		map[uuid.UUID]int64{cream.ID: 2400, bacon.ID: 2400, eggs.ID: 48},
		dishes, day, day, 3*day)

	require.Len(t, advice, 3)
	byName := map[string]SupplyAdvice{}
	order := make([]string, len(advice))
	for i, a := range advice {
		byName[a.Product.Name], order[i] = a, a.Product.Name
	}
	require.Equal(t, []string{"Соль", "Яйцо", "Сливки"}, order, "what is missing first, then by hours of cover")

	low := byName["Сливки"]
	require.Equal(t, LowCover, low.Reason)
	require.InDelta(t, 6.0, *low.CoverHours, 1e-9)
	require.EqualValues(t, 6600, low.Suggested, "100 per hour for 72 hours minus 600 in stock")
	require.EqualValues(t, 2400, low.Consumed)
	require.Equal(t, []catalog.Dish{pasta}, low.NeededBy)

	expired := byName["Яйцо"]
	require.Equal(t, OutOfStock, expired.Reason, "expired stock is not a reserve")
	require.InDelta(t, 0.0, *expired.CoverHours, 1e-9)
	require.EqualValues(t, 144, expired.Suggested, "2 per hour for 72 hours")

	missing := byName["Соль"]
	require.Equal(t, OutOfStock, missing.Reason)
	require.Nil(t, missing.CoverHours, "there was no consumption to estimate from")
	require.EqualValues(t, 20, missing.Suggested, "ten portions of the dish that needs the most")
	require.Equal(t, []catalog.Dish{pasta, omelette}, missing.NeededBy)

	t.Run("enough stock is not advised", func(t *testing.T) {
		require.Empty(t, supplyAdvice([]inventory.StockLevel{level(cream, 2400, 0)},
			map[uuid.UUID]int64{cream.ID: 2400}, dishes, day, day, 3*day), "exactly the cover is enough")
	})
	t.Run("the level is a parameter", func(t *testing.T) {
		advice := supplyAdvice([]inventory.StockLevel{level(bacon, 5000, 0)},
			map[uuid.UUID]int64{bacon.ID: 2400}, dishes, day, 3*day, 7*day)
		require.Len(t, advice, 1)
		require.EqualValues(t, 100*168-5000, advice[0].Suggested)
	})
	// Расход делится на время, за которое есть данные: 2000 за 10 минут - это 12000 в час,
	// а не 2000 / 24 = 83, при которых остатка "хватало" на 36 часов.
	t.Run("short history", func(t *testing.T) {
		observed := observedPeriod(now, now.Add(-10*time.Minute), day)
		advice := supplyAdvice([]inventory.StockLevel{level(cream, 3000, 0)},
			map[uuid.UUID]int64{cream.ID: 2000}, dishes, observed, day, 3*day)
		require.Len(t, advice, 1)
		require.InDelta(t, 0.25, *advice[0].CoverHours, 1e-9, "3000 at 12000 per hour")
		require.EqualValues(t, 12000*72-3000, advice[0].Suggested)
	})
	// Остатка не хватает на порцию: блюдо в стоп-листе, хотя остаток не нулевой и расхода не было.
	t.Run("less than a portion without consumption", func(t *testing.T) {
		advice := supplyAdvice([]inventory.StockLevel{level(salt, 1, 0)}, nil, dishes, day, day, 3*day)
		require.Len(t, advice, 1)
		require.Equal(t, OutOfStock, advice[0].Reason)
		require.Nil(t, advice[0].CoverHours)
		require.Equal(t, []catalog.Dish{pasta, omelette}, advice[0].NeededBy)
	})
	t.Run("a portion of every dish without consumption is enough", func(t *testing.T) {
		require.Empty(t, supplyAdvice([]inventory.StockLevel{level(salt, 2, 0)}, nil, dishes, day, day, 3*day),
			"the most demanding dish needs 2")
	})
}

func TestObservedPeriod(t *testing.T) {
	const day = 24 * time.Hour
	tests := []struct {
		name  string
		first time.Time
		want  time.Duration
	}{
		{"short history", now.Add(-30 * time.Minute), 30 * time.Minute},
		{"history shorter than the minimum", now.Add(-time.Minute), minObservedPeriod},
		{"history fills the period", now.Add(-day), day},
		{"no consumption", time.Time{}, day},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, observedPeriod(now, tt.first, day))
		})
	}
}
