package orders

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/eventstore"
)

// ErrNotFound - заказа с таким идентификатором нет.
var ErrNotFound = errors.New("orders: not found")

// ValidationError - некорректное поле входных данных команды.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("orders: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение: по этому методу
// слой API распознаёт ошибку валидации, не зная о типах модулей.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// MaxPortions - верхняя граница числа порций одного блюда в заказе. Вместе с catalog.MaxQuantity
// она не даёт потребности заказа (количество на порцию, умноженное на порции) переполнить int64.
const MaxPortions int64 = 10_000

// MaxCancelCommentLength - самый длинный комментарий к отмене заказа, в символах.
const MaxCancelCommentLength = 1000

// cancelComment обрезает комментарий к отмене по краям и проверяет длину.
func cancelComment(comment string) (string, error) {
	comment = strings.TrimSpace(comment)
	if len([]rune(comment)) > MaxCancelCommentLength {
		return "", invalid("comment", fmt.Sprintf("must not exceed %d characters", MaxCancelCommentLength))
	}
	return comment, nil
}

// UnknownDishError - в заказе указано блюдо, которого нет в каталоге компании точки.
type UnknownDishError struct {
	DishID uuid.UUID
}

func (e *UnknownDishError) Error() string {
	return fmt.Sprintf("orders: unknown dish %s", e.DishID)
}

// DishNotSoldError - блюдо выключено на точке, заказать его на ней нельзя.
type DishNotSoldError struct {
	DishID uuid.UUID
}

func (e *DishNotSoldError) Error() string {
	return fmt.Sprintf("orders: dish %s is not sold on the point", e.DishID)
}

// DishWithoutRecipeError - блюду ещё не задана рецептура, заказать его нельзя.
type DishWithoutRecipeError struct {
	DishID uuid.UUID
}

func (e *DishWithoutRecipeError) Error() string {
	return fmt.Sprintf("orders: dish %s has no recipe", e.DishID)
}

type OrderStatus string

const (
	OrderAcceptedStatus  OrderStatus = "accepted"
	OrderRejectedStatus  OrderStatus = "rejected"
	OrderCancelledStatus OrderStatus = "cancelled"
)

// ErrNotCancellable - отменить можно только принятый заказ: отклонённый остатков не менял,
// отменённый уже отменён.
var ErrNotCancellable = errors.New("orders: only an accepted order can be cancelled")

// Order - результат оформления заказа. Отказ из-за нехватки продуктов - тоже результат:
// он записан событием и возвращается со статусом "rejected" и списком нехваток.
type Order struct {
	ID      uuid.UUID   `json:"id"`
	PointID uuid.UUID   `json:"point_id"`
	Status  OrderStatus `json:"status"`
	// PlacedAt - время оформления: момент выполнения команды, он же время события заказа.
	PlacedAt time.Time `json:"placed_at"`
	// CancelledAt - время отмены, только у отменённого заказа.
	CancelledAt *time.Time  `json:"cancelled_at,omitempty"`
	Items       []OrderItem `json:"items"`
	Shortages   []Shortage  `json:"shortages,omitempty"`
	// PlacedBy - кто оформил заказ: по нему видно заказы Мигома и действует правило отмены.
	PlacedBy eventstore.Actor `json:"placed_by"`
	// CancelledBy и CancelComment - кто отменил и с каким комментарием, только у отменённого.
	CancelledBy   *eventstore.Actor `json:"cancelled_by,omitempty"`
	CancelComment string            `json:"cancel_comment,omitempty"`
}

func validateItems(items []OrderItem) error {
	if len(items) == 0 {
		return invalid("items", "must not be empty")
	}
	seen := make(map[uuid.UUID]bool, len(items))
	for _, item := range items {
		switch {
		case item.Portions <= 0:
			return invalid("items", fmt.Sprintf("portions of dish %s must be positive", item.DishID))
		case item.Portions > MaxPortions:
			return invalid("items", fmt.Sprintf("portions of dish %s must not exceed %d", item.DishID, MaxPortions))
		case seen[item.DishID]:
			return invalid("items", fmt.Sprintf("dish %s is listed twice", item.DishID))
		}
		seen[item.DishID] = true
	}
	return nil
}

// requirements считает потребность заказа в продуктах: количество на порцию, умноженное
// на число порций, с суммированием продукта по всем блюдам. Результат упорядочен по продукту.
func requirements(dishes map[uuid.UUID]catalog.Dish, items []OrderItem) ([]inventory.Need, error) {
	total := make(map[uuid.UUID]int64)
	for _, item := range items {
		dish := dishes[item.DishID]
		if len(dish.Ingredients) == 0 {
			return nil, &DishWithoutRecipeError{DishID: item.DishID}
		}
		for _, ingredient := range dish.Ingredients {
			total[ingredient.ProductID] += ingredient.Quantity * item.Portions
		}
	}

	needs := make([]inventory.Need, 0, len(total))
	for productID, quantity := range total {
		needs = append(needs, inventory.Need{ProductID: productID, Quantity: quantity})
	}
	slices.SortFunc(needs, func(a, b inventory.Need) int {
		return cmp.Compare(a.ProductID.String(), b.ProductID.String())
	})
	return needs, nil
}
