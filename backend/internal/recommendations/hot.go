// Package recommendations подсказывает, какие блюда продавать в первую очередь, чтобы продукты
// с истекающим сроком не ушли в списание, и какие продукты пора заказывать.
//
// Оба расчёта выполняются при чтении: они зависят от текущего времени. Своя таблица у модуля одна:
// ручные горячие предложения точек (manual.go).
package recommendations

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/stoplist"
)

// RiskReason - причина горячего предложения: продукт блюда, у которого истекает срок годности.
// Urgency, Coverage, Significance и Exclusivity - множители вклада продукта в приоритет, каждый от 0 до 1.
type RiskReason struct {
	Product catalog.Product
	// AtRisk - сумма остатков партий продукта, истекающих в пределах горизонта.
	AtRisk int64
	// NearestBatchID и NearestExpiresAt - партия с ближайшим сроком: её заказ израсходует первой (FEFO).
	NearestBatchID   uuid.UUID
	NearestExpiresAt time.Time
	// PerPortion - количество продукта на порцию блюда.
	PerPortion int64
	// Dishes - сколько доступных блюд используют продукт.
	Dishes int
	// Urgency: чем меньше осталось до срока ближайшей партии, тем ближе к 1.
	Urgency float64
	// Coverage: какую долю количества под риском блюдо способно израсходовать.
	Coverage float64
	// Significance: меньше 1, если ближайшей партии не хватает на одну порцию.
	Significance float64
	// Exclusivity: 1 у единственного потребителя продукта, стремится к 0.5 с ростом числа блюд.
	Exclusivity float64
}

// contribution - вклад продукта в приоритет блюда.
func (r RiskReason) contribution() float64 {
	return r.Urgency * r.Coverage * r.Significance * r.Exclusivity
}

// DishOffer - горячее предложение: блюдо, продажу которого стоит стимулировать.
type DishOffer struct {
	Dish catalog.Dish
	// Priority - сумма вкладов продуктов под риском, умноженная на 100.
	Priority int
	// Portions - сколько порций можно приготовить.
	Portions int64
	// SuggestedPortions - сколько порций стоит продать, чтобы израсходовать количество под риском.
	SuggestedPortions int64
	// Reasons - продукты под риском от самого весомого.
	Reasons []RiskReason
	// Manual - ручная настройка, если блюдо добавлено управляющим.
	Manual *HotOfferSetting
}

// productRisk - партии продукта, истекающие в пределах горизонта.
type productRisk struct {
	product catalog.Product
	atRisk  int64
	nearest inventory.Batch
	// nearestQuantity - остаток партий с ближайшим сроком: срочность и значимость описывают его.
	// Среднее по всем партиям гасило бы срочность: большая поздняя партия перевешивала ту,
	// что пропадёт через час.
	nearestQuantity int64
}

// hotOffers считает рейтинг блюд по меню и партиям под риском.
//
// Блюдо попадает в рейтинг, если его можно приготовить и хотя бы один его ингредиент есть в партии,
// истекающей в пределах horizon. Приоритет - сумма по таким ингредиентам:
//
//	вклад = срочность * охват * значимость * исключительность
//
// Все множители лежат от 0 до 1 и не зависят от единицы измерения продукта, поэтому граммы,
// миллилитры и штуки сравнимы, а каждый множитель можно показать как объяснение.
func hotOffers(menu []stoplist.Availability, expiring []inventory.ExpiringBatch, now time.Time, horizon time.Duration) []DishOffer {
	risks := map[uuid.UUID]*productRisk{}
	for _, e := range expiring {
		left := e.Batch.ExpiresAt.Sub(now)
		if e.Expired || left <= 0 || left > horizon || e.Batch.Quantity <= 0 {
			continue // просроченное продавать нельзя, а за горизонтом риска ещё нет
		}
		risk := risks[e.Batch.ProductID]
		if risk == nil {
			risk = &productRisk{product: e.Product, nearest: e.Batch}
			risks[e.Batch.ProductID] = risk
		}
		switch {
		case e.Batch.ExpiresAt.Before(risk.nearest.ExpiresAt):
			risk.nearest, risk.nearestQuantity = e.Batch, e.Batch.Quantity
		case e.Batch.ExpiresAt.Equal(risk.nearest.ExpiresAt):
			risk.nearestQuantity += e.Batch.Quantity
		}
		risk.atRisk += e.Batch.Quantity
	}

	// сколько доступных блюд используют продукт: чем их больше, тем менее критично каждое
	consumers := map[uuid.UUID]int{}
	for _, a := range menu {
		if !a.Available() {
			continue
		}
		for _, ingredient := range a.Dish.Ingredients {
			consumers[ingredient.ProductID]++
		}
	}

	var offers []DishOffer
	for _, a := range menu {
		if !a.Available() {
			continue // блюдо из стоп-листа продать нельзя
		}
		offer := DishOffer{Dish: a.Dish, Portions: a.Portions}
		score := 0.0
		for _, ingredient := range a.Dish.Ingredients {
			risk := risks[ingredient.ProductID]
			if risk == nil {
				continue
			}
			perPortion, atRisk := float64(ingredient.Quantity), float64(risk.atRisk)
			reason := RiskReason{
				Product: risk.product, AtRisk: risk.atRisk,
				NearestBatchID: risk.nearest.ID, NearestExpiresAt: risk.nearest.ExpiresAt,
				PerPortion: ingredient.Quantity, Dishes: consumers[ingredient.ProductID],
				Urgency:      1 - float64(risk.nearest.ExpiresAt.Sub(now))/float64(horizon),
				Coverage:     min(1, float64(a.Portions)*perPortion/atRisk),
				Significance: min(1, float64(risk.nearestQuantity)/perPortion),
				Exclusivity:  (1 + 1/float64(consumers[ingredient.ProductID])) / 2,
			}
			score += reason.contribution()
			offer.Reasons = append(offer.Reasons, reason)
			// порций, чтобы израсходовать количество под риском, но не больше, чем можно приготовить
			toUseUp := (risk.atRisk + ingredient.Quantity - 1) / ingredient.Quantity
			offer.SuggestedPortions = max(offer.SuggestedPortions, min(a.Portions, toUseUp))
		}
		if len(offer.Reasons) == 0 {
			continue
		}
		offer.Priority = int(math.Round(score * 100))
		slices.SortStableFunc(offer.Reasons, func(x, y RiskReason) int { return cmp.Compare(y.contribution(), x.contribution()) })
		offers = append(offers, offer)
	}

	slices.SortStableFunc(offers, func(x, y DishOffer) int {
		return cmp.Or(cmp.Compare(y.Priority, x.Priority), cmp.Compare(x.Dish.Name, y.Dish.Name))
	})
	return offers
}
