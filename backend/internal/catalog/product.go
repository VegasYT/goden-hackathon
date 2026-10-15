package catalog

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrNotFound - продукта или блюда с таким идентификатором нет.
	ErrNotFound = errors.New("catalog: not found")
	// ErrNameTaken - название уже занято другим продуктом или блюдом. Команды возвращают
	// NameTakenError, errors.Is с ним совпадает.
	ErrNameTaken = errors.New("catalog: name is already taken")
	// ErrUnitLocked - единицу продукта уже нельзя менять: он есть в рецептуре, поставке или на складе.
	ErrUnitLocked = errors.New("catalog: product unit can no longer change")
)

// NameTakenError - название занято продуктом или блюдом компании; Archived - архивным.
type NameTakenError struct {
	What     string
	Name     string
	Archived bool
}

func (e *NameTakenError) Error() string {
	return fmt.Sprintf("%s %q: %s", e.What, e.Name, ErrNameTaken)
}

func (e *NameTakenError) Is(target error) bool { return target == ErrNameTaken }

// FieldDetail - вид ошибки для поля name: занято работающим или архивным.
func (e *NameTakenError) FieldDetail() (code string, limit *int64) {
	if e.Archived {
		return "taken_by_archived", nil
	}
	return "taken", nil
}

// ArchivedProductError - продукт в архиве: его нельзя брать в новые поставки и рецептуры.
type ArchivedProductError struct {
	ProductID uuid.UUID
	Name      string
}

func (e *ArchivedProductError) Error() string {
	return fmt.Sprintf("catalog: product %s %q is archived", e.ProductID, e.Name)
}

// ProductInUseError - продукт есть в рецептурах неархивных блюд, его нельзя отправить в архив.
type ProductInUseError struct {
	Dishes []string
}

func (e *ProductInUseError) Error() string {
	return "catalog: product is used in recipes of " + strings.Join(e.Dishes, ", ")
}

// ArchivedIngredientsError - в рецептуре блюда есть архивные продукты, его нельзя восстановить.
type ArchivedIngredientsError struct {
	Products []string
}

func (e *ArchivedIngredientsError) Error() string {
	return "catalog: recipe has archived products " + strings.Join(e.Products, ", ")
}

// ValidationError - некорректное поле входных данных команды.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("catalog: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение: по этому методу
// слой API распознаёт ошибку валидации, не зная о типах модулей.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

const (
	// MaxQuantity - верхняя граница количества продукта в любой операции, в минимальной единице
	// продукта (миллион тонн в граммах). Без неё суммы остатков и потребность заказа
	// переполняют int64: границу проверяют все модули, которые принимают количество.
	MaxQuantity int64 = 1_000_000_000_000
	// MaxShelfLifeDays - верхняя граница срока хранения продукта (десять лет).
	MaxShelfLifeDays = 3650
)

// Product - продукт каталога. Единица измерения после создания не меняется:
// в ней выражены все количества продукта в поставках, рецептурах и остатках.
type Product struct {
	ID            uuid.UUID `json:"id"`
	CompanyID     uuid.UUID `json:"company_id"`
	Name          string    `json:"name"`
	Unit          Unit      `json:"unit"`
	Category      string    `json:"category"`
	ShelfLifeDays *int      `json:"shelf_life_days,omitempty"`
	Archived      bool      `json:"archived"`
}

// ProductUse - где используется продукт: для ответов каталога.
type ProductUse struct {
	// Dishes - неархивные блюда с продуктом в рецептуре.
	Dishes int64
	// UnitLocked - продукт был в рецептуре, поставке или на складе: единицу уже не сменить.
	UnitLocked bool
}

// ProductChange - изменение продукта: nil оставляет поле прежним. ClearShelfLife снимает срок хранения.
type ProductChange struct {
	Name           *string
	Unit           *Unit
	Category       *string
	ShelfLifeDays  *int
	ClearShelfLife bool
}

// applied возвращает состав продукта после изменения.
func (c ProductChange) applied(p Product) NewProduct {
	next := NewProduct{Name: p.Name, Unit: p.Unit, Category: p.Category, ShelfLifeDays: p.ShelfLifeDays}
	if c.Name != nil {
		next.Name = *c.Name
	}
	if c.Unit != nil {
		next.Unit = *c.Unit
	}
	if c.Category != nil {
		next.Category = *c.Category
	}
	switch {
	case c.ClearShelfLife:
		next.ShelfLifeDays = nil
	case c.ShelfLifeDays != nil:
		next.ShelfLifeDays = c.ShelfLifeDays
	}
	return next
}

// sameAs сообщает, совпадает ли состав с продуктом: такое изменение события не пишет.
func (p NewProduct) sameAs(product Product) bool {
	sameShelfLife := (p.ShelfLifeDays == nil) == (product.ShelfLifeDays == nil) &&
		(p.ShelfLifeDays == nil || *p.ShelfLifeDays == *product.ShelfLifeDays)
	return p.Name == product.Name && p.Unit == product.Unit && p.Category == product.Category && sameShelfLife
}

// NewProduct - данные для создания продукта.
type NewProduct struct {
	Name          string
	Unit          Unit
	Category      string
	ShelfLifeDays *int
}

// normalized обрезает пробелы и проверяет данные.
func (p NewProduct) normalized() (NewProduct, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Category = strings.TrimSpace(p.Category)
	switch {
	case p.Name == "":
		return p, invalid("name", "must not be empty")
	case p.Unit != UnitGram && p.Unit != UnitMilliliter && p.Unit != UnitPiece:
		return p, invalid("unit", "must be one of g, ml, pcs")
	case p.ShelfLifeDays != nil && *p.ShelfLifeDays <= 0:
		return p, invalid("shelf_life_days", "must be positive")
	case p.ShelfLifeDays != nil && *p.ShelfLifeDays > MaxShelfLifeDays:
		return p, invalid("shelf_life_days", fmt.Sprintf("must not exceed %d", MaxShelfLifeDays))
	}
	return p, nil
}

// nameKey - название в виде, в котором проверяется уникальность: без учёта регистра.
// Принимает название с уже обрезанными пробелами.
func nameKey(name string) string {
	return strings.ToLower(name)
}
