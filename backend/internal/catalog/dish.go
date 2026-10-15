package catalog

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Dish - блюдо с рецептурой. У только что созданного блюда рецептура пустая.
type Dish struct {
	ID          uuid.UUID    `json:"id"`
	CompanyID   uuid.UUID    `json:"company_id"`
	Name        string       `json:"name"`
	Ingredients []Ingredient `json:"ingredients"`
	Archived    bool         `json:"archived"`
}

// Ingredient - количество продукта на одну порцию в единице измерения продукта. Название,
// единицу и архив продукта заполняет чтение каталога; в командах значимы продукт и количество.
type Ingredient struct {
	ProductID       uuid.UUID `json:"product_id"`
	Quantity        int64     `json:"quantity"`
	ProductName     string    `json:"product_name,omitempty"`
	Unit            Unit      `json:"unit,omitempty"`
	ProductArchived bool      `json:"product_archived,omitempty"`
}

// ListFilter - фильтры списков продуктов и блюд каталога.
type ListFilter struct {
	// Query - часть названия без учёта регистра.
	Query    string
	Archived bool
	// Category - только для продуктов: точное совпадение категории.
	Category string
	// ByCategory - только для продуктов: по категории, без категории в конце.
	ByCategory bool
	// MissingRecipe - только для блюд: без рецептуры.
	MissingRecipe bool
}

// namePart - часть названия для LIKE: в нижнем регистре, знаки шаблона экранированы
// и действуют как обычные символы.
func namePart(query string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(nameKey(strings.TrimSpace(query)))
}

// PointDish - блюдо компании на точке: продаётся ли оно там.
type PointDish struct {
	DishID  uuid.UUID `json:"dish_id"`
	Name    string    `json:"name"`
	Enabled bool      `json:"enabled"`
}

// UnknownProductError - в рецептуре указан продукт, которого нет в каталоге компании блюда.
type UnknownProductError struct {
	ProductID uuid.UUID
}

func (e *UnknownProductError) Error() string {
	return fmt.Sprintf("catalog: unknown product %s", e.ProductID)
}

func normalizedDishName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalid("name", "must not be empty")
	}
	return name, nil
}

// validateRecipe проверяет состав, не обращаясь к базе: существование продуктов проверяет команда.
func validateRecipe(ingredients []Ingredient) error {
	if len(ingredients) == 0 {
		return invalid("ingredients", "must not be empty")
	}
	seen := make(map[uuid.UUID]bool, len(ingredients))
	for _, ing := range ingredients {
		switch {
		case ing.Quantity <= 0:
			return invalid("ingredients", fmt.Sprintf("quantity of product %s must be positive", ing.ProductID))
		case ing.Quantity > MaxQuantity:
			return invalid("ingredients", fmt.Sprintf("quantity of product %s must not exceed %d", ing.ProductID, MaxQuantity))
		case seen[ing.ProductID]:
			return invalid("ingredients", fmt.Sprintf("product %s is listed twice", ing.ProductID))
		}
		seen[ing.ProductID] = true
	}
	return nil
}
