//go:build integration

package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func hotDishes(t *testing.T, s apitest.Server) []oas.DishRecommendation {
	t.Helper()
	page, err := s.Client.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.EqualValues(t, len(page.Items), page.Total)
	return page.Items
}

func deliveryAdvice(t *testing.T, s apitest.Server) map[string]oas.DeliveryRecommendation {
	t.Helper()
	page, err := s.Client.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	out := make(map[string]oas.DeliveryRecommendation, len(page.Items))
	for _, item := range page.Items {
		out[item.Product.Name] = item
	}
	return out
}

func TestHotOffersThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	sauce := dishWith(t, s, "Соус", per(cream, 100))
	dishWith(t, s, "Бекон жареный", per(bacon, 50))
	creamBatch := supply(t, s, cream, 500, oct(15))
	supply(t, s, bacon, 1000, oct(25))

	// 10 октября: до 15-го больше 72 часов, под риском ничего нет
	require.Empty(t, hotDishes(t, s))
	page, err := s.Client.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.NotNil(t, page.Items, "an empty list, not null")

	// горизонт пошире - партия уже под риском
	wide, err := s.Client.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID, HorizonHours: oas.NewOptInt32(240)})
	require.NoError(t, err)
	require.Len(t, wide.Items, 1)

	// время идёт: партия входит в горизонт, приоритет растёт
	s.Clock.Set(oct(13))
	offers := hotDishes(t, s)
	require.Len(t, offers, 1)
	first := offers[0]
	require.Equal(t, sauce.ID, first.Dish.ID)
	require.EqualValues(t, 5, first.AvailablePortions)
	require.EqualValues(t, 5, first.SuggestedPortions)
	require.Len(t, first.Reasons, 1)
	reason := first.Reasons[0]
	require.Equal(t, *cream, reason.Product)
	require.EqualValues(t, 500, reason.AtRiskQuantity)
	require.Equal(t, creamBatch, reason.BatchID, "the reason names the batch")
	require.Equal(t, oct(15), reason.ExpiresAt)
	require.EqualValues(t, 48, reason.HoursLeft)
	require.EqualValues(t, 100, reason.RequiredQuantity)
	require.EqualValues(t, 1, reason.DishesUsing)
	require.InDelta(t, 1.0/3, reason.Urgency, 1e-9, "48 of 72 hours are left")
	require.EqualValues(t, 33, first.Priority)

	s.Clock.Set(oct(14).Add(12 * time.Hour))
	later := hotDishes(t, s)
	require.Greater(t, later[0].Priority, first.Priority, "the rating changes as the expiry gets closer")
	require.EqualValues(t, 12, later[0].Reasons[0].HoursLeft)

	// проданные порции расходуют именно партию под риском: предложение исчезает
	placed, err := s.Client.PlaceOrder(ctx, order(portions(sauce, 5)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, placed.Status)
	require.Empty(t, hotDishes(t, s))

	// срок истёк: просроченное не рекомендуется
	supply(t, s, cream, 300, oct(15))
	require.Len(t, hotDishes(t, s), 1)
	s.Clock.Set(oct(15))
	require.Empty(t, hotDishes(t, s))
}

func TestDeliveryRecommendationsThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream, bacon, salt := create(t, s, "Сливки 33%"), create(t, s, "Бекон"), create(t, s, "Соль")
	create(t, s, "Салфетки")
	sauce := dishWith(t, s, "Соус", per(cream, 100), per(salt, 1))
	dishWith(t, s, "Бекон жареный", per(bacon, 50))
	supply(t, s, salt, 1000, oct(30))
	supply(t, s, cream, 3000, oct(30))

	// бекона нет, а он нужен блюду; расхода ещё не было
	advice := deliveryAdvice(t, s)
	require.Len(t, advice, 1)
	missing := advice["Бекон"]
	require.Equal(t, oas.DeliveryRecommendationReasonOutOfStock, missing.Reason)
	require.False(t, missing.CoverHours.Set)
	require.EqualValues(t, 500, missing.SuggestedQuantity, "ten portions of 50")
	require.Equal(t, "Бекон жареный", missing.NeededBy[0].Name)

	// заказы за последние сутки: 24 порции, сливок израсходовано 2400, осталось 600
	s.Clock.Advance(time.Hour)
	placed, err := s.Client.PlaceOrder(ctx, order(portions(sauce, 24)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, placed.Status)

	// расход только что начался: темп считается по времени, за которое есть данные (не меньше
	// 10 минут), а не по всему периоду - 2400 за 10 минут, остатка хватит на две с половиной минуты
	fresh := deliveryAdvice(t, s)["Сливки 33%"]
	require.InDelta(t, 600.0/(2400*6), fresh.CoverHours.Value, 1e-9)

	// через сутки история покрывает весь период: 2400 за 24 часа
	s.Clock.Advance(24 * time.Hour)
	advice = deliveryAdvice(t, s)
	low := advice["Сливки 33%"]
	require.Equal(t, oas.DeliveryRecommendationReasonLowCover, low.Reason)
	require.EqualValues(t, 600, low.AvailableQuantity)
	require.EqualValues(t, 2400, low.ConsumedQuantity)
	require.InDelta(t, 6.0, low.CoverHours.Value, 1e-9, "100 per hour, 600 in stock")
	require.EqualValues(t, 6600, low.SuggestedQuantity)
	require.NotContains(t, advice, "Соль", "976 in stock at 1 per hour is plenty")
	require.NotContains(t, advice, "Салфетки", "nobody needs them")

	page, err := s.Client.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, []string{"Бекон", "Сливки 33%"}, []string{page.Items[0].Product.Name, page.Items[1].Product.Name},
		"what is missing first")

	// поставка закрывает рекомендацию
	supply(t, s, cream, 7000, oct(30))
	supply(t, s, bacon, 500, oct(30))
	require.Empty(t, deliveryAdvice(t, s))

	// расход старше периода не учитывается
	narrow, err := s.Client.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{
		PointId:     s.PointID,
		WindowHours: oas.NewOptInt32(1), CoverHours: oas.NewOptInt32(100), TargetHours: oas.NewOptInt32(200),
	})
	require.NoError(t, err)
	require.Empty(t, narrow.Items, "the order was placed a day ago, outside the last hour")
}

func TestRecommendationParameters(t *testing.T) {
	s := apitest.New(t)
	for _, path := range []string{
		"/recommendations/dishes?horizon_hours=0", "/recommendations/dishes?horizon_hours=-3",
		"/recommendations/deliveries?window_hours=0", "/recommendations/deliveries?cover_hours=abc",
	} {
		resp, body := raw(t, s, http.MethodGet, pointPath(s)+path, "", nil)
		requireRawProblem(t, resp, body, http.StatusBadRequest)
	}

	_, err := s.Client.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{
		PointId:    s.PointID,
		CoverHours: oas.NewOptInt32(48), TargetHours: oas.NewOptInt32(24),
	})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "target_hours", problem.Errors[0].Field)

	// официант видит горячие предложения, повар - нет
	waiter := s.Staff(t, "waiter", oas.RoleWaiter)
	_, err = waiter.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	cook := s.Staff(t, "cook", oas.RoleCook)
	_, err = cook.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	requireProblem(t, err, http.StatusForbidden)
}
