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

func hotOfferSettings(t *testing.T, client *oas.Client, s apitest.Server) []oas.HotOfferSetting {
	t.Helper()
	out, err := client.ListHotOfferSettings(ctx, oas.ListHotOfferSettingsParams{PointId: s.PointID})
	require.NoError(t, err)
	return out.Items
}

func TestHotOffersSetByHand(t *testing.T) {
	s := apitest.New(t)
	manager := s.Staff(t, "manager", oas.RoleManager)
	waiter := s.Staff(t, "waiter", oas.RoleWaiter)
	cream, sugar := create(t, s, "Сливки"), create(t, s, "Сахар")
	sauce := dishWith(t, s, "Соус", per(cream, 100))
	tiramisu := dishWith(t, s, "Тирамису", per(sugar, 50))
	supply(t, s, cream, 500, oct(15))
	supply(t, s, sugar, 100, oct(25))
	s.Clock.Set(oct(13)) // в Москве 03:00, сливки истекают через двое суток

	set := func(client *oas.Client, dish *oas.Dish, change oas.HotOfferChange) (*oas.HotOfferSetting, error) {
		return client.SetHotOffer(ctx, &change, oas.SetHotOfferParams{PointId: s.PointID, DishId: dish.ID})
	}
	names := func() []string {
		var out []string
		for _, offer := range hotDishes(t, s) {
			out = append(out, offer.Dish.Name)
		}
		return out
	}
	require.Equal(t, []string{"Соус"}, names())

	// добавленное вручную блюдо без риска идёт первым, с комментарием и автором
	pinned, err := set(manager, tiramisu, oas.HotOfferChange{Mode: oas.HotOfferModePinned, Comment: oas.NewOptString(" новинка ")})
	require.NoError(t, err)
	require.Equal(t, "новинка", pinned.Comment)
	require.Equal(t, "manager", pinned.SetBy.Name.Value)
	offers := hotDishes(t, s)
	require.Equal(t, []string{"Тирамису", "Соус"}, names())
	require.Equal(t, "новинка", offers[0].Manual.Value.Comment)
	require.Equal(t, "manager", offers[0].Manual.Value.SetBy.Name.Value)
	require.Empty(t, offers[0].Reasons)
	require.False(t, offers[1].Manual.Set)

	// ручное блюдо с продуктом под риском сохраняет причины
	s.Clock.Advance(time.Minute) // ручные идут в порядке добавления
	_, err = set(manager, sauce, oas.HotOfferChange{Mode: oas.HotOfferModePinned})
	require.NoError(t, err)
	offers = hotDishes(t, s)
	require.Equal(t, []string{"Тирамису", "Соус"}, names())
	require.True(t, offers[1].Manual.Set)
	require.NotEmpty(t, offers[1].Reasons)

	// скрытие действует до конца дня точки
	_, err = set(manager, sauce, oas.HotOfferChange{Mode: oas.HotOfferModeHidden})
	requireFieldError(t, requireProblem(t, err, http.StatusUnprocessableEntity), "time_zone", "")
	hidden, err := set(manager, sauce, oas.HotOfferChange{Mode: oas.HotOfferModeHidden, TimeZone: oas.NewOptString("Europe/Moscow")})
	require.NoError(t, err)
	endOfDay := oct(13).Add(21 * time.Hour) // полночь 14 октября по Москве
	require.Equal(t, endOfDay, hidden.HiddenUntil.Value)
	require.Equal(t, []string{"Тирамису"}, names())
	settings := hotOfferSettings(t, waiter, s)
	require.Len(t, settings, 2)
	require.Equal(t, oas.HotOfferModeHidden, settings[1].Mode)
	require.Equal(t, "Соус", settings[1].Dish.Name)

	s.Clock.Set(endOfDay)
	require.Equal(t, []string{"Тирамису", "Соус"}, names(), "the next day the computation decides again")
	require.Len(t, hotOfferSettings(t, waiter, s), 1)

	// ручное блюдо в стоп-листе не показывается, а настройка остаётся
	placed, err := s.Client.PlaceOrder(ctx, order(portions(tiramisu, 2)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, placed.Status)
	require.Equal(t, []string{"Соус"}, names())
	supply(t, s, sugar, 100, oct(25))
	require.Equal(t, []string{"Тирамису", "Соус"}, names())

	// возврат к расчёту
	_, err = set(manager, tiramisu, oas.HotOfferChange{Mode: oas.HotOfferModeAuto})
	require.NoError(t, err)
	require.Equal(t, []string{"Соус"}, names())
	require.Empty(t, hotOfferSettings(t, waiter, s))

	// официант предложения не меняет, блюдо другой компании не найдено
	_, err = set(waiter, sauce, oas.HotOfferChange{Mode: oas.HotOfferModePinned})
	requireForbidden(t, err, "menu.manage")
	foreign := ownDish(t, newNetwork(t, s), "Чужое")
	_, err = set(s.Client, foreign, oas.HotOfferChange{Mode: oas.HotOfferModePinned})
	requireProblem(t, err, http.StatusNotFound)

	// на отключённой точке режим не меняется
	_, err = s.Client.DisablePoint(ctx, oas.DisablePointParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = set(s.Client, sauce, oas.HotOfferChange{Mode: oas.HotOfferModePinned})
	requireProblem(t, err, http.StatusConflict)
}
