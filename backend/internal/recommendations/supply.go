package recommendations

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
)

// SupplyReason - почему продукт пора заказывать.
type SupplyReason string

const (
	// OutOfStock - доступного остатка нет, а продукт нужен блюдам или расходуется.
	OutOfStock SupplyReason = "out_of_stock"
	// LowCover - при расходе за последний период остатка хватит меньше чем на заданный запас по времени.
	LowCover SupplyReason = "low_cover"
)

// minObservedPeriod не даёт одному заказу сразу после запуска превратиться в огромный темп расхода.
const minObservedPeriod = 10 * time.Minute

// observedPeriod - время, за которое есть данные о расходе: от самого раннего расхода в периоде
// расчёта до текущего момента, в пределах от minObservedPeriod до длины периода. Расход делится
// на него, а не на весь период: иначе на свежей системе темп занижается в десятки раз.
// То же правило - в прогнозе стоп-листа (internal/analytics/forecast.go).
func observedPeriod(now, firstConsumption time.Time, window time.Duration) time.Duration {
	if firstConsumption.IsZero() {
		return window // расхода не было, делить нечего
	}
	return min(max(now.Sub(firstConsumption), minObservedPeriod), window)
}

// порций самого требовательного блюда, на которые заказывается продукт без истории расхода:
// ориентир, а не расчёт - без спроса точнее не посчитать
const portionsWithoutHistory = 10

// SupplyAdvice - рекомендация поставки продукта.
type SupplyAdvice struct {
	Product catalog.Product
	// Available - доступный остаток: партии с неистёкшим сроком годности.
	Available int64
	// Consumed - расход под заказы за период расчёта.
	Consumed int64
	// CoverHours - на сколько часов хватит остатка при таком расходе; nil, если расхода не было.
	CoverHours *float64
	// Suggested - рекомендуемое количество поставки.
	Suggested int64
	Reason    SupplyReason
	// NeededBy - блюда, в рецептуру которых входит продукт.
	NeededBy []catalog.Dish
}

// supplyAdvice решает, какие продукты пора заказывать.
//
// Уровень, ниже которого предлагается поставка, задан не количеством, а запасом по времени:
// остатка хватит меньше чем на cover при расходе за время наблюдения observed (см. observedPeriod).
// Количество доводит запас до target.
// Расход - только списания под заказы: порча и корректировки спросом не являются.
func supplyAdvice(
	stock []inventory.StockLevel, consumed map[uuid.UUID]int64, dishes []catalog.Dish, observed, cover, target time.Duration,
) []SupplyAdvice {
	neededBy := map[uuid.UUID][]catalog.Dish{}
	perPortion := map[uuid.UUID]int64{} // наибольшее количество на порцию среди блюд с этим продуктом
	for _, dish := range dishes {
		for _, ingredient := range dish.Ingredients {
			neededBy[ingredient.ProductID] = append(neededBy[ingredient.ProductID], dish)
			perPortion[ingredient.ProductID] = max(perPortion[ingredient.ProductID], ingredient.Quantity)
		}
	}

	var advice []SupplyAdvice
	for _, level := range stock {
		id := level.Product.ID
		a := SupplyAdvice{Product: level.Product, Available: level.Available, Consumed: consumed[id], NeededBy: neededBy[id]}
		switch {
		case a.Consumed > 0:
			perHour := float64(a.Consumed) / observed.Hours()
			coverHours := float64(a.Available) / perHour
			if coverHours >= cover.Hours() {
				continue
			}
			a.CoverHours = &coverHours
			a.Suggested = max(int64(math.Ceil(perHour*target.Hours()))-a.Available, 1)
			a.Reason = LowCover
			if a.Available == 0 {
				a.Reason = OutOfStock
			}
		// остатка не хватает на порцию хотя бы одного блюда: оно в стоп-листе, хотя остаток не нулевой
		case a.Available < perPortion[id] && len(a.NeededBy) > 0:
			a.Reason, a.Suggested = OutOfStock, perPortion[id]*portionsWithoutHistory
		default:
			continue // запаса хватает либо продукт никому не нужен
		}
		advice = append(advice, a)
	}

	// от самых срочных: сначала то, чего нет, затем по времени до исчерпания
	hours := func(a SupplyAdvice) float64 {
		if a.Available == 0 || a.CoverHours == nil {
			return 0
		}
		return *a.CoverHours
	}
	slices.SortStableFunc(advice, func(x, y SupplyAdvice) int {
		return cmp.Or(cmp.Compare(hours(x), hours(y)), cmp.Compare(x.Product.Name, y.Product.Name))
	})
	return advice
}
