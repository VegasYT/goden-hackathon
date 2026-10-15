package recommendations

import (
	"context"
	"errors"
	"time"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
)

// HTTP реализует операции рекомендаций из api/openapi.yaml.
type HTTP struct {
	svc    *Service
	points *organization.Service
	dishes *catalog.Service
	now    func() time.Time
}

func NewHTTP(svc *Service, points *organization.Service, dishes *catalog.Service) *HTTP {
	return &HTTP{svc: svc, points: points, dishes: dishes, now: svc.now}
}

func hours(n int32) time.Duration { return time.Duration(n) * time.Hour }

// ListDishRecommendations реализует GET /recommendations/dishes.
func (h *HTTP) ListDishRecommendations(
	ctx context.Context, params oas.ListDishRecommendationsParams,
) (*oas.DishRecommendationPage, error) {
	// значения по умолчанию и границы параметров проверяет сгенерированный разбор запроса
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	offers, total, err := h.svc.DishOffers(ctx, point, hours(params.HorizonHours.Value),
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	now := h.now()
	page := &oas.DishRecommendationPage{Items: make([]oas.DishRecommendation, len(offers)), Total: total}
	var actors []*oas.Actor
	for i, offer := range offers {
		item := oas.DishRecommendation{
			Dish:     oas.DishRef{ID: offer.Dish.ID, Name: offer.Dish.Name},
			Priority: int32(offer.Priority), AvailablePortions: offer.Portions, SuggestedPortions: offer.SuggestedPortions,
			Reasons: make([]oas.RecommendationReason, len(offer.Reasons)),
		}
		for j, r := range offer.Reasons {
			item.Reasons[j] = oas.RecommendationReason{
				Product: *catalog.ProductResponse(r.Product), AtRiskQuantity: r.AtRisk,
				BatchID: r.NearestBatchID, ExpiresAt: r.NearestExpiresAt,
				HoursLeft:        int32(r.NearestExpiresAt.Sub(now) / time.Hour),
				RequiredQuantity: r.PerPortion, DishesUsing: int32(r.Dishes),
				Urgency: r.Urgency, Coverage: r.Coverage, Significance: r.Significance, Exclusivity: r.Exclusivity,
			}
		}
		page.Items[i] = item
		if m := offer.Manual; m != nil {
			page.Items[i].Manual = oas.NewOptManualOffer(oas.ManualOffer{Comment: m.Comment, SetBy: actorResponse(m.SetBy), SetAt: m.SetAt})
			actors = append(actors, &page.Items[i].Manual.Value.SetBy)
		}
	}
	return page, h.points.WithNames(ctx, actors)
}

// ListHotOfferSettings реализует GET /recommendations/dishes/settings.
func (h *HTTP) ListHotOfferSettings(ctx context.Context, params oas.ListHotOfferSettingsParams) (*oas.HotOfferSettings, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	settings, err := h.svc.HotOfferSettings(ctx, point.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.HotOfferSettings{Items: make([]oas.HotOfferSetting, len(settings))}
	actors := make([]*oas.Actor, len(settings))
	for i, setting := range settings {
		dish, err := h.dishes.Dish(ctx, setting.DishID)
		if err != nil {
			return nil, err
		}
		out.Items[i] = settingResponse(setting, dish.Name)
		actors[i] = &out.Items[i].SetBy
	}
	return out, h.points.WithNames(ctx, actors)
}

// SetHotOffer реализует PUT /recommendations/dishes/{dishId}.
func (h *HTTP) SetHotOffer(ctx context.Context, req *oas.HotOfferChange, params oas.SetHotOfferParams) (*oas.HotOfferSetting, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("recommendations: request has no actor")
	}
	point, err := h.points.WorkingPoint(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	dish, err := h.dishes.DishOf(ctx, point.CompanyID, params.DishId)
	if err != nil {
		return nil, err
	}
	setting, err := h.svc.SetHotOffer(ctx, actor, params.IdempotencyKey.Or(""), point.ID, dish.ID,
		HotOfferMode(req.Mode), req.Comment.Or(""), req.TimeZone.Or(""))
	if err != nil {
		return nil, err
	}
	out := settingResponse(setting, dish.Name)
	return &out, h.points.WithNames(ctx, []*oas.Actor{&out.SetBy})
}

func settingResponse(s HotOfferSetting, dishName string) oas.HotOfferSetting {
	out := oas.HotOfferSetting{
		Dish: oas.DishRef{ID: s.DishID, Name: dishName}, Mode: oas.HotOfferMode(s.Mode),
		Comment: s.Comment, SetBy: actorResponse(s.SetBy), SetAt: s.SetAt,
	}
	if s.HiddenUntil != nil {
		out.HiddenUntil = oas.NewOptDateTime(*s.HiddenUntil)
	}
	return out
}

func actorResponse(a eventstore.Actor) oas.Actor {
	return oas.Actor{Type: oas.ActorType(a.Type), ID: a.ID}
}

// ListDeliveryRecommendations реализует GET /recommendations/deliveries.
func (h *HTTP) ListDeliveryRecommendations(
	ctx context.Context, params oas.ListDeliveryRecommendationsParams,
) (*oas.DeliveryRecommendationPage, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	advice, total, err := h.svc.SupplyAdvice(ctx, point,
		hours(params.WindowHours.Value), hours(params.CoverHours.Value), hours(params.TargetHours.Value),
		catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.DeliveryRecommendationPage{Items: make([]oas.DeliveryRecommendation, len(advice)), Total: total}
	for i, a := range advice {
		item := oas.DeliveryRecommendation{
			Product:           *catalog.ProductResponse(a.Product),
			AvailableQuantity: a.Available, ConsumedQuantity: a.Consumed, SuggestedQuantity: a.Suggested,
			Reason:   oas.DeliveryRecommendationReason(a.Reason),
			NeededBy: make([]oas.DishRef, len(a.NeededBy)),
		}
		if a.CoverHours != nil {
			item.CoverHours = oas.NewOptFloat64(*a.CoverHours)
		}
		for j, dish := range a.NeededBy {
			item.NeededBy[j] = oas.DishRef{ID: dish.ID, Name: dish.Name}
		}
		page.Items[i] = item
	}
	return page, nil
}
