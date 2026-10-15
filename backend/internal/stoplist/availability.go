// Package stoplist считает доступность блюд: сколько порций можно приготовить из текущих
// остатков и каких продуктов не хватает.
//
// Стоп-лист не хранится: он считается при каждом запросе по синхронным таблицам рецептур и
// партий, тем же правилом доступного остатка, что и оформление заказа. Поэтому он не отстаёт
// от записи и учитывает просрочку, которая наступает без события (ADR-002).
package stoplist

import (
	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
)

// Reason - почему блюдо нельзя приготовить.
type Reason string

const (
	// NoRecipe - блюду не задана рецептура.
	NoRecipe Reason = "no_recipe"
	// InsufficientStock - хотя бы одного продукта не хватает на порцию.
	InsufficientStock Reason = "insufficient_stock"
)

// LimitingIngredient - ингредиент, который ограничивает число порций блюда.
type LimitingIngredient struct {
	Product catalog.Product
	// Required - количество на одну порцию.
	Required int64
	// Available - доступный остаток продукта: партии с неистёкшим сроком годности.
	Available int64
	// Expired - остаток просроченных партий: объясняет, почему продукт есть, но недоступен.
	Expired int64
}

// Availability - доступность блюда на момент расчёта.
type Availability struct {
	Dish catalog.Dish
	// Portions - сколько порций можно приготовить; ноль означает, что блюдо в стоп-листе.
	Portions int64
	// Reason заполнена только у недоступного блюда.
	Reason Reason
	// Limiting: у недоступного блюда - все ингредиенты, которых не хватает на порцию,
	// у доступного - ингредиенты, которые определяют число порций.
	Limiting []LimitingIngredient
	// Ingredients - вся рецептура в её порядке с остатком каждого продукта.
	Ingredients []LimitingIngredient
}

// Available сообщает, можно ли приготовить хотя бы одну порцию.
func (a Availability) Available() bool { return a.Portions > 0 }

// availability считает доступность блюда по остаткам продуктов. Продукта, которого нет в stock,
// на складе нет. Число порций - минимум по ингредиентам от "доступно / на порцию": это то же
// сравнение потребности с доступным остатком, которое делает оформление заказа.
func availability(dish catalog.Dish, stock map[uuid.UUID]inventory.StockLevel) Availability {
	if len(dish.Ingredients) == 0 {
		return Availability{Dish: dish, Reason: NoRecipe}
	}

	result := Availability{Dish: dish, Portions: -1}
	for _, ingredient := range dish.Ingredients {
		level := stock[ingredient.ProductID]
		portions := level.Available / ingredient.Quantity
		limiting := LimitingIngredient{
			Product: level.Product, Required: ingredient.Quantity, Available: level.Available, Expired: level.Expired,
		}
		if limiting.Product.ID == uuid.Nil {
			limiting.Product.ID = ingredient.ProductID
		}
		result.Ingredients = append(result.Ingredients, limiting)
		switch {
		case result.Portions < 0 || portions < result.Portions:
			result.Portions, result.Limiting = portions, []LimitingIngredient{limiting}
		case portions == result.Portions:
			result.Limiting = append(result.Limiting, limiting)
		}
	}
	if result.Portions == 0 {
		result.Reason = InsufficientStock
	}
	return result
}
