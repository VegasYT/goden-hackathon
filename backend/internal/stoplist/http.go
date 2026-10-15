package stoplist

import (
	"context"

	"github.com/google/uuid"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/organization"
)

// HTTP реализует операции меню и стоп-листа из api/openapi.yaml.
type HTTP struct {
	svc    *Service
	points *organization.Service
}

func NewHTTP(svc *Service, points *organization.Service) *HTTP {
	return &HTTP{svc: svc, points: points}
}

// GetMenu реализует GET /menu: все блюда с доступностью.
func (h *HTTP) GetMenu(ctx context.Context, params oas.GetMenuParams) (*oas.DishAvailabilityPage, error) {
	// значения по умолчанию подставляет сгенерированный разбор запроса
	return h.page(ctx, params.PointId, false, catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
}

// GetStopList реализует GET /stop-list: блюда, которые сейчас нельзя приготовить.
func (h *HTTP) GetStopList(ctx context.Context, params oas.GetStopListParams) (*oas.DishAvailabilityPage, error) {
	return h.page(ctx, params.PointId, true, catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
}

func (h *HTTP) page(ctx context.Context, pointID uuid.UUID, onlyStopped bool, page catalog.Page) (*oas.DishAvailabilityPage, error) {
	point, err := h.points.Point(ctx, pointID)
	if err != nil {
		return nil, err
	}
	menu, total, err := h.svc.Menu(ctx, point, onlyStopped, page)
	if err != nil {
		return nil, err
	}
	out := &oas.DishAvailabilityPage{Items: make([]oas.DishAvailability, len(menu)), Total: total}
	for i, a := range menu {
		item := oas.DishAvailability{
			Dish:      oas.DishRef{ID: a.Dish.ID, Name: a.Dish.Name},
			Available: a.Available(), Portions: a.Portions,
			// пустой список, а не null: поле обязательное
			Limiting: ingredientsResponse(a.Limiting), Ingredients: ingredientsResponse(a.Ingredients),
		}
		if a.Reason != "" {
			item.Reason = oas.NewOptDishAvailabilityReason(oas.DishAvailabilityReason(a.Reason))
		}
		out.Items[i] = item
	}
	return out, nil
}

func ingredientsResponse(ingredients []LimitingIngredient) []oas.LimitingIngredient {
	out := make([]oas.LimitingIngredient, len(ingredients))
	for i, l := range ingredients {
		out[i] = oas.LimitingIngredient{
			Product:          *catalog.ProductResponse(l.Product),
			RequiredQuantity: l.Required, AvailableQuantity: l.Available, ExpiredQuantity: l.Expired,
		}
	}
	return out
}
