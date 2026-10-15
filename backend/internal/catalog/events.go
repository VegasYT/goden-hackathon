// Package catalog ведёт справочники: продукты, блюда и рецептуры.
package catalog

import (
	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

func init() {
	eventstore.Register[ProductCreated]()
	eventstore.Register[DishCreated]()
	eventstore.Register[RecipeChanged]()
	eventstore.Register[DishAvailabilityChanged]()
	eventstore.Register[ProductChanged]()
	eventstore.Register[ProductArchived]()
	eventstore.Register[ProductRestored]()
	eventstore.Register[DishRenamed]()
	eventstore.Register[DishArchived]()
	eventstore.Register[DishRestored]()
}

// ProductStream - поток справочных данных продукта. Остатки продукта лежат в inventory.StockStream.
func ProductStream(productID uuid.UUID) string { return "product-" + productID.String() }

// DishStream - поток блюда и его рецептуры.
func DishStream(dishID uuid.UUID) string { return "dish-" + dishID.String() }

// MenuStream - поток настроек меню точки: какие блюда компании на ней не продаются.
func MenuStream(pointID uuid.UUID) string { return "menu-" + pointID.String() }

// Unit - единица измерения продукта. Количество всегда целое число в этой единице.
type Unit string

const (
	UnitGram       Unit = "g"
	UnitMilliliter Unit = "ml"
	UnitPiece      Unit = "pcs"
)

func (u *Unit) UnmarshalText(text []byte) error {
	return eventstore.ParseEnum(u, text, "unit", UnitGram, UnitMilliliter, UnitPiece)
}

type ProductCreated struct {
	ProductID uuid.UUID `json:"product_id"`
	// компания, которой принадлежит продукт; не меняется (ADR-013)
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
	Unit      Unit      `json:"unit"`
	Category  string    `json:"category"`
	// срок хранения нужен только для проверки согласованности дат поставки (ADR-010)
	ShelfLifeDays *int `json:"shelf_life_days,omitempty"`
}

func (ProductCreated) EventType() string  { return "ProductCreated" }
func (ProductCreated) SchemaVersion() int { return 1 }

type DishCreated struct {
	DishID    uuid.UUID `json:"dish_id"`
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
}

func (DishCreated) EventType() string  { return "DishCreated" }
func (DishCreated) SchemaVersion() int { return 1 }

// RecipeChanged содержит полный состав блюда после изменения, а не разницу.
type RecipeChanged struct {
	DishID uuid.UUID `json:"dish_id"`
	// компания блюда: по ней сигнал об изменении рецептуры доходит до всех её точек
	CompanyID   uuid.UUID          `json:"company_id"`
	Ingredients []RecipeIngredient `json:"ingredients"`
}

func (RecipeChanged) EventType() string  { return "RecipeChanged" }
func (RecipeChanged) SchemaVersion() int { return 1 }

// DishAvailabilityChanged - блюдо выключили или включили на точке. Без этого события
// блюдо компании продаётся на каждой её точке.
type DishAvailabilityChanged struct {
	PointID uuid.UUID `json:"point_id"`
	DishID  uuid.UUID `json:"dish_id"`
	Enabled bool      `json:"enabled"`
}

func (DishAvailabilityChanged) EventType() string  { return "DishAvailabilityChanged" }
func (DishAvailabilityChanged) SchemaVersion() int { return 1 }

// ProductChanged содержит полный состав продукта после изменения, а не разницу:
// перестроению не нужно собирать продукт по кускам.
type ProductChanged struct {
	ProductID     uuid.UUID `json:"product_id"`
	CompanyID     uuid.UUID `json:"company_id"`
	Name          string    `json:"name"`
	Unit          Unit      `json:"unit"`
	Category      string    `json:"category"`
	ShelfLifeDays *int      `json:"shelf_life_days,omitempty"`
}

func (ProductChanged) EventType() string  { return "ProductChanged" }
func (ProductChanged) SchemaVersion() int { return 1 }

// ProductArchived - продукт убран из работы: его нет в новых поставках, рецептурах и рекомендациях.
type ProductArchived struct {
	ProductID uuid.UUID `json:"product_id"`
	CompanyID uuid.UUID `json:"company_id"`
}

func (ProductArchived) EventType() string  { return "ProductArchived" }
func (ProductArchived) SchemaVersion() int { return 1 }

type ProductRestored struct {
	ProductID uuid.UUID `json:"product_id"`
	CompanyID uuid.UUID `json:"company_id"`
}

func (ProductRestored) EventType() string  { return "ProductRestored" }
func (ProductRestored) SchemaVersion() int { return 1 }

type DishRenamed struct {
	DishID    uuid.UUID `json:"dish_id"`
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
}

func (DishRenamed) EventType() string  { return "DishRenamed" }
func (DishRenamed) SchemaVersion() int { return 1 }

// DishArchived - блюдо не продаётся ни на одной точке компании. Выключение блюда на точках
// при этом не меняется и действует после восстановления.
type DishArchived struct {
	DishID    uuid.UUID `json:"dish_id"`
	CompanyID uuid.UUID `json:"company_id"`
}

func (DishArchived) EventType() string  { return "DishArchived" }
func (DishArchived) SchemaVersion() int { return 1 }

type DishRestored struct {
	DishID    uuid.UUID `json:"dish_id"`
	CompanyID uuid.UUID `json:"company_id"`
}

func (DishRestored) EventType() string  { return "DishRestored" }
func (DishRestored) SchemaVersion() int { return 1 }

// RecipeIngredient - количество продукта на одну порцию.
type RecipeIngredient struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int64     `json:"quantity"`
}
