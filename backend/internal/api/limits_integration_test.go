//go:build integration

package api_test

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
)

// Приёмка, в которой сумма частей переполняет int64 и совпадает с заявленным количеством,
// раньше создавала партию на 9223372036854775807 единиц. Запрос идёт в обход клиента:
// сгенерированный клиент такое число не отправит.
func TestApprovalWithOverflowingQuantitiesIsRejected(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 10, oct(20)))

	body := fmt.Sprintf(`{"items":[{"item_id":%q,"accepted_quantity":%d,"rejected_quantity":%d,"quarantined_quantity":12,
		"reject_reason":"damaged","quarantine_reason":"check","comment":"c"}]}`,
		registered.Items[0].ID, int64(math.MaxInt64), int64(math.MaxInt64))
	resp, _ := raw(t, s, http.MethodPost, fmt.Sprintf("%s/deliveries/%s/approval", pointPath(s), registered.ID), body, nil)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Empty(t, quantities(t, s, cream), "no batch is created")
	delivery, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, oas.DeliveryStatusRegistered, delivery.Status)
}

// Число порций, при котором потребность (200 на порцию) переполнялась до 184 и заказ принимался.
func TestOrderWithOverflowingPortionsIsRejected(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dish := dishWith(t, s, "Соус", per(cream, 200))
	supply(t, s, cream, 1000, oct(20))

	for _, portions := range []int64{92233720368547759, 50000000000000000, 10001} {
		body := fmt.Sprintf(`{"items":[{"dish_id":%q,"portions":%d}]}`, dish.ID, portions)
		resp, _ := raw(t, s, http.MethodPost, pointPath(s)+"/orders", body, nil)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "portions %d", portions)
	}

	require.Equal(t, []int64{1000}, quantities(t, s, cream), "stock is untouched")
	s.Sync(t)
	orders, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Empty(t, orders.Items)
}

// Несколько партий на границе количества: суммы остатков помещаются в int64,
// и чтения, которые их складывают, отвечают.
func TestReadsWorkWithBatchesAtTheQuantityLimit(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dishWith(t, s, "Соус", per(cream, 200))
	for range 3 {
		supply(t, s, cream, catalog.MaxQuantity, oct(20))
	}

	stock, err := s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Len(t, stock.Items, 1)
	require.Equal(t, 3*catalog.MaxQuantity, stock.Items[0].AvailableQuantity)

	m := menu(t, s)
	require.Equal(t, 3*catalog.MaxQuantity/200, m["Соус"].Portions)

	_, err = s.Client.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)

	// выше границы поставку не зарегистрировать
	body := fmt.Sprintf(`{"number":"N","delivered_at":"2026-10-10T09:00:00Z","items":[{"product_id":%q,"declared_quantity":%d,"expires_at":"2026-10-20T00:00:00Z"}]}`,
		cream.ID, catalog.MaxQuantity+1)
	resp, _ := raw(t, s, http.MethodPost, pointPath(s)+"/deliveries", body, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// Длина строк ограничена контрактом: раньше принималось название из ста тысяч символов.
func TestTooLongStringsAreRejected(t *testing.T) {
	s := apitest.New(t)

	body := fmt.Sprintf(`{"name":%q,"unit":"g"}`, strings.Repeat("я", 201))
	resp, _ := raw(t, s, http.MethodPost, companyPath(s)+"/products", body, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body = fmt.Sprintf(`{"name":%q,"unit":"g"}`, strings.Repeat("я", 200))
	resp, _ = raw(t, s, http.MethodPost, companyPath(s)+"/products", body, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "200 characters, not bytes, are allowed")

	// нулевой символ в названии - ошибка запроса, а не внутренняя ошибка
	resp, _ = raw(t, s, http.MethodPost, companyPath(s)+"/products", `{"name":"a`+"\\"+`u0000b","unit":"g"}`, nil)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
