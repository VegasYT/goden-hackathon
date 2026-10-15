package analytics

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/analytics/db"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/organization"
)

var all = catalog.Page{Limit: math.MaxInt32}

// ForecastCause - почему продукта не хватит на порцию.
type ForecastCause string

const (
	// CauseConsumption - остаток израсходуют.
	CauseConsumption ForecastCause = "consumption"
	// CauseExpiry - после истечения срока партии остатка станет меньше порции.
	CauseExpiry ForecastCause = "expiry"
)

// StopListForecast - прогноз блюда с объяснением по ограничивающему продукту.
type StopListForecast struct {
	Dish       catalog.Dish
	Product    catalog.Product
	ExpectedAt time.Time
	// Available - годный остаток продукта сейчас, Required - продукта на порцию.
	Available int64
	Required  int64
	// ConsumptionPerHour - темп расхода, по которому построен прогноз.
	ConsumptionPerHour float64
	Cause              ForecastCause
}

// Forecast строит прогноз стоп-листа точки: по её расходу, её партиям и блюдам её компании,
// которые на точке продаются.
func (s *Service) Forecast(ctx context.Context, point organization.Point, window, horizon time.Duration) ([]StopListForecast, error) {
	now := s.now().UTC()
	rows, err := db.New(s.pool).ConsumptionSince(ctx, db.ConsumptionSinceParams{
		PointID: point.ID, FromTime: now.Add(-window), ToTime: now,
	})
	if err != nil {
		return nil, fmt.Errorf("analytics: consumption: %w", err)
	}
	consumed := make(map[uuid.UUID]int64, len(rows))
	for _, row := range rows {
		consumed[row.ProductID] = row.Quantity
	}
	first, err := db.New(s.pool).FirstConsumptionSince(ctx, db.FirstConsumptionSinceParams{
		PointID: point.ID, FromTime: now.Add(-window), ToTime: now,
	})
	if err != nil {
		return nil, fmt.Errorf("analytics: first consumption: %w", err)
	}
	dishes, _, err := s.products.DishesOnPoint(ctx, point.CompanyID, point.ID, all)
	if err != nil {
		return nil, err
	}
	productIDs := ingredientIDs(dishes)
	products, err := s.products.ProductsByID(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	batches := make(map[uuid.UUID][]inventory.Batch, len(productIDs))
	for _, productID := range productIDs {
		batches[productID], err = s.stock.Batches(ctx, point.ID, productID)
		if err != nil {
			return nil, err
		}
	}
	return buildForecasts(now, observedPeriod(now, first, window), horizon, dishes, products, batches, consumed), nil
}

// minObservedPeriod не даёт одному заказу сразу после запуска превратиться в огромный темп расхода.
const minObservedPeriod = 10 * time.Minute

// observedPeriod - время, за которое есть данные о расходе: от самого раннего расхода в окне
// до текущего момента, в пределах от minObservedPeriod до длины окна. Расход делится на него,
// а не на всё окно: иначе на свежей системе темп занижается в десятки раз.
func observedPeriod(now, firstConsumption time.Time, window time.Duration) time.Duration {
	return min(max(now.Sub(firstConsumption), minObservedPeriod), window)
}

func ingredientIDs(dishes []catalog.Dish) []uuid.UUID {
	set := map[uuid.UUID]bool{}
	for _, dish := range dishes {
		for _, ingredient := range dish.Ingredients {
			set[ingredient.ProductID] = true
		}
	}
	ids := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

func buildForecasts(
	now time.Time, observed, horizon time.Duration, dishes []catalog.Dish, products map[uuid.UUID]catalog.Product,
	batches map[uuid.UUID][]inventory.Batch, consumed map[uuid.UUID]int64,
) []StopListForecast {
	var result []StopListForecast
	for _, dish := range dishes {
		if len(dish.Ingredients) == 0 {
			continue
		}
		available := true
		var candidate *StopListForecast
		for _, ingredient := range dish.Ingredients {
			current := availableQuantity(now, batches[ingredient.ProductID])
			if current < ingredient.Quantity {
				available = false
				break
			}
			at, cause, ok := productForecast(now, observed, horizon, ingredient.Quantity,
				consumed[ingredient.ProductID], batches[ingredient.ProductID])
			if !ok {
				continue
			}
			product := products[ingredient.ProductID]
			next := StopListForecast{
				Dish: dish, Product: product, ExpectedAt: at,
				Available: current, Required: ingredient.Quantity, Cause: cause,
				ConsumptionPerHour: max(float64(consumed[ingredient.ProductID]), 0) / observed.Hours(),
			}
			if candidate == nil || at.Before(candidate.ExpectedAt) ||
				(at.Equal(candidate.ExpectedAt) && product.Name < candidate.Product.Name) {
				candidate = &next
			}
		}
		if available && candidate != nil {
			result = append(result, *candidate)
		}
	}
	slices.SortFunc(result, func(a, b StopListForecast) int {
		return cmp.Or(a.ExpectedAt.Compare(b.ExpectedAt), cmp.Compare(a.Dish.Name, b.Dish.Name))
	})
	return result
}

func availableQuantity(now time.Time, batches []inventory.Batch) int64 {
	var quantity int64
	for _, batch := range batches {
		if batch.Quantity > 0 && batch.ExpiresAt.After(now) {
			quantity += batch.Quantity
		}
	}
	return quantity
}

type simulatedBatch struct {
	quantity  float64
	expiresAt time.Time
}

// productForecast моделирует равномерный расход по FEFO. В момент срока годности
// неизрасходованный остаток партии исчезает целиком. Причина - что оставило остаток
// меньше порции: расход между сроками или истечение партии.
func productForecast(
	now time.Time, observed, horizon time.Duration, required, consumed int64, batches []inventory.Batch,
) (time.Time, ForecastCause, bool) {
	if observed <= 0 || horizon <= 0 {
		return time.Time{}, "", false
	}
	state := make([]simulatedBatch, 0, len(batches))
	for _, batch := range batches {
		if batch.Quantity > 0 && batch.ExpiresAt.After(now) {
			state = append(state, simulatedBatch{quantity: float64(batch.Quantity), expiresAt: batch.ExpiresAt.UTC()})
		}
	}
	// без расхода темп нулевой: остаток убывает только по истечению сроков годности
	rate := max(float64(consumed), 0) / observed.Seconds()
	end, cursor := now.Add(horizon), now
	for len(state) > 0 && !state[0].expiresAt.After(end) {
		expiresAt := state[0].expiresAt
		if at, ok := depletionBefore(cursor, expiresAt, total(state), float64(required), rate); ok {
			return at, CauseConsumption, true
		}
		consumeFEFO(state, rate*expiresAt.Sub(cursor).Seconds())
		cursor = expiresAt
		cut := 0
		for cut < len(state) && !state[cut].expiresAt.After(expiresAt) {
			cut++
		}
		state = state[cut:]
		if total(state) < float64(required) {
			return cursor, CauseExpiry, true
		}
	}
	at, ok := depletionBefore(cursor, end, total(state), float64(required), rate)
	return at, CauseConsumption, ok
}

func depletionBefore(from, to time.Time, available, required, rate float64) (time.Time, bool) {
	if rate <= 0 {
		return time.Time{}, false // расхода нет: до следующего истечения срока остаток не меняется
	}
	seconds := (available - required) / rate
	if seconds < 0 {
		return time.Time{}, false
	}
	if seconds <= to.Sub(from).Seconds() {
		return from.Add(time.Duration(seconds * float64(time.Second))), true
	}
	return time.Time{}, false
}

func consumeFEFO(batches []simulatedBatch, quantity float64) {
	for i := range batches {
		taken := min(batches[i].quantity, quantity)
		batches[i].quantity -= taken
		quantity -= taken
		if quantity <= 0 {
			return
		}
	}
}

func total(batches []simulatedBatch) float64 {
	var quantity float64
	for _, batch := range batches {
		quantity += batch.quantity
	}
	return quantity
}
