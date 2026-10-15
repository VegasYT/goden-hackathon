//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/eventstore"
)

func TestGeneratorSettingsThroughAPI(t *testing.T) {
	s := apitest.New(t)

	initial, err := s.Client.GetGeneratorSettings(ctx, oas.GetGeneratorSettingsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.False(t, initial.Running, "a new point starts with the generator stopped")
	require.False(t, initial.UpdatedAt.Set)
	require.Positive(t, initial.DeliveriesPerMinute)
	require.Positive(t, initial.OrdersPerMinute)

	updated, err := s.Client.UpdateGeneratorSettings(ctx, &oas.GeneratorSettings{
		Running: true, DeliveriesPerMinute: 12, OrdersPerMinute: 60,
	}, oas.UpdateGeneratorSettingsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.True(t, updated.Running)
	require.Equal(t, int32(12), updated.DeliveriesPerMinute)
	require.Equal(t, int32(60), updated.OrdersPerMinute)
	require.Equal(t, s.Clock.Now(), updated.UpdatedAt.Value)

	read, err := s.Client.GetGeneratorSettings(ctx, oas.GetGeneratorSettingsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, updated, read)

	for name, body := range map[string]string{
		"rate above the limit": `{"running":true,"deliveries_per_minute":1,"orders_per_minute":1000}`,
		"negative rate":        `{"running":true,"deliveries_per_minute":-1,"orders_per_minute":1}`,
		"missing field":        `{"running":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, data := raw(t, s, http.MethodPut, pointPath(s)+"/generator", body, nil)
			requireRawProblem(t, resp, data, http.StatusBadRequest)
		})
	}
	read, err = s.Client.GetGeneratorSettings(ctx, oas.GetGeneratorSettingsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, updated, read, "rejected requests do not change settings")
}

// Настройки генератора у каждой точки свои, меняет их только владелец. Учётная запись
// генератора получает права на точке, только пока он на ней включён.
func TestGeneratorIsEnabledPerPoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	manager, _ := hire(t, s, k.owner, k.a, "manager", oas.RoleManager)
	on := &oas.GeneratorSettings{Running: true, DeliveriesPerMinute: 6, OrdersPerMinute: 30}
	account := s.As(t, eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"}, "generator.run")
	newDelivery := &oas.NewDelivery{Number: "G-1", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 50, oct(15))}}

	// управляющий генератором не распоряжается
	_, err := manager.UpdateGeneratorSettings(ctx, on, oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
	requireForbidden(t, err, "generator.manage")
	_, err = manager.GetGeneratorSettings(ctx, oas.GetGeneratorSettingsParams{PointId: k.a.ID})
	requireForbidden(t, err, "generator.manage")

	// пока генератор выключен, у его учётной записи прав на точке нет
	points, err := account.ListGeneratorPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, points)
	_, err = account.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireForbidden(t, err, "deliveries.register")
	_, err = account.ListProducts(ctx, oas.ListProductsParams{CompanyId: k.company.ID})
	requireForbidden(t, err, "catalog.view")

	_, err = k.owner.UpdateGeneratorSettings(ctx, on, oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
	require.NoError(t, err)
	other, err := k.owner.GetGeneratorSettings(ctx, oas.GetGeneratorSettingsParams{PointId: k.b.ID})
	require.NoError(t, err)
	require.False(t, other.Running, "the neighbouring point keeps its own settings")

	points, err = account.ListGeneratorPoints(ctx)
	require.NoError(t, err)
	require.Equal(t, []oas.GeneratorPoint{{PointID: k.a.ID, CompanyID: k.company.ID, DeliveriesPerMinute: 6, OrdersPerMinute: 30}}, points)
	registered, err := account.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.a.ID})
	require.NoError(t, err)
	_, err = account.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: k.a.ID, DeliveryId: registered.ID})
	require.NoError(t, err)
	_, err = account.ListDishes(ctx, oas.ListDishesParams{CompanyId: k.company.ID})
	require.NoError(t, err)
	// на соседней точке генератор не включён, а лишних прав на включённой у него нет
	_, err = account.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.b.ID})
	requireForbidden(t, err, "deliveries.register")
	_, err = account.GetStock(ctx, oas.GetStockParams{PointId: k.a.ID})
	requireForbidden(t, err, "inventory.view")
	_, err = account.DisablePoint(ctx, oas.DisablePointParams{PointId: k.a.ID})
	requireForbidden(t, err, "company.manage")

	// список точек генератора не виден пользователям, даже владельцу
	_, err = k.owner.ListGeneratorPoints(ctx)
	requireForbidden(t, err, "generator.run")

	// выключение отзывает права сразу
	_, err = k.owner.UpdateGeneratorSettings(ctx, &oas.GeneratorSettings{Running: false, DeliveriesPerMinute: 6, OrdersPerMinute: 30},
		oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
	require.NoError(t, err)
	_, err = account.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireForbidden(t, err, "deliveries.register")
	points, err = account.ListGeneratorPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, points)
}

// Точку отключили при включённом генераторе: он перестаёт на ней работать, а владелец
// может его выключить. Отключение необратимо, поэтому иначе генератор было бы нечем остановить.
func TestGeneratorOnADisabledPoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	on := &oas.GeneratorSettings{Running: true, DeliveriesPerMinute: 6, OrdersPerMinute: 30}
	off := &oas.GeneratorSettings{Running: false, DeliveriesPerMinute: 6, OrdersPerMinute: 30}
	account := s.As(t, eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"}, "generator.run")
	for _, point := range []*oas.Point{k.a, k.b} {
		_, err := k.owner.UpdateGeneratorSettings(ctx, on, oas.UpdateGeneratorSettingsParams{PointId: point.ID})
		require.NoError(t, err)
	}
	_, err := k.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: k.a.ID})
	require.NoError(t, err)

	// отключённой точки нет в списке генератора, прав на ней у него нет
	points, err := account.ListGeneratorPoints(ctx)
	require.NoError(t, err)
	require.Len(t, points, 1)
	require.Equal(t, k.b.ID, points[0].PointID)
	newDelivery := &oas.NewDelivery{Number: "G-1", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 50, oct(15))}}
	_, err = account.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireForbidden(t, err, "deliveries.register")
	_, err = account.RegisterDelivery(ctx, newDelivery, oas.RegisterDeliveryParams{PointId: k.b.ID})
	require.NoError(t, err)

	// выключить можно, включить обратно - нет
	stopped, err := k.owner.UpdateGeneratorSettings(ctx, off, oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.False(t, stopped.Running)
	_, err = k.owner.UpdateGeneratorSettings(ctx, on, oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
	require.Equal(t, "conflict", requireProblem(t, err, http.StatusConflict).Type)
	read, err := k.owner.GetGeneratorSettings(ctx, oas.GetGeneratorSettingsParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.False(t, read.Running)

	// компанию отключили: генератор не работает ни на одной её точке и не читает её каталог
	_, err = k.owner.DisableCompany(ctx, oas.DisableCompanyParams{CompanyId: k.company.ID})
	require.NoError(t, err)
	points, err = account.ListGeneratorPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, points)
	_, err = account.ListProducts(ctx, oas.ListProductsParams{CompanyId: k.company.ID})
	requireForbidden(t, err, "catalog.view")
	_, err = k.owner.UpdateGeneratorSettings(ctx, off, oas.UpdateGeneratorSettingsParams{PointId: k.b.ID})
	require.NoError(t, err)
}

// Генератору нужно знать, какие блюда продаются на точке: заказ с выключенным блюдом отклоняется.
func TestGeneratorReadsDishesOfItsPoint(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	account := s.As(t, eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"}, "generator.run")
	_, err := account.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: k.a.ID})
	requireForbidden(t, err, "catalog.view")

	_, err = k.owner.UpdateGeneratorSettings(ctx, &oas.GeneratorSettings{Running: true, DeliveriesPerMinute: 6, OrdersPerMinute: 30},
		oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
	require.NoError(t, err)
	dishes, err := account.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, dishes.Total)
	_, err = account.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: k.b.ID})
	requireForbidden(t, err, "catalog.view")
}

// В режиме без проверки токенов генератор ходит без токена, то есть как пользователь dev:
// на точке с включённым генератором dev получает его права и в чужой компании.
func TestDevUserActsAsGeneratorWhereItIsEnabled(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	register := func() error {
		_, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
			Number: "G-1", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(k.cream, 50, oct(15))},
		}, oas.RegisterDeliveryParams{PointId: k.a.ID})
		return err
	}
	settings := func(running bool) {
		_, err := k.owner.UpdateGeneratorSettings(ctx, &oas.GeneratorSettings{Running: running, DeliveriesPerMinute: 6, OrdersPerMinute: 30},
			oas.UpdateGeneratorSettingsParams{PointId: k.a.ID})
		require.NoError(t, err)
	}

	requireForbidden(t, register(), "deliveries.register")
	settings(true)
	require.NoError(t, register())
	settings(false)
	requireForbidden(t, register(), "deliveries.register")
}
