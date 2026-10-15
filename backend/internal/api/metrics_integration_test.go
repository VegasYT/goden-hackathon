//go:build integration

package api_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func TestMetricsEndpoint(t *testing.T) {
	s := apitest.New(t)

	// без токена и вне префикса API
	resp, err := http.Get(s.URL + "/metrics")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
	require.Contains(t, string(body), "go_goroutines")

	require.EqualValues(t, 10, s.Metric(t, "goden_db_pool_connections", "state", "max"), "the configured pool limit")
	require.Equal(t, 3, s.MetricSeries(t, "goden_db_pool_connections"))
	require.Equal(t, 1, s.MetricSeries(t, "goden_db_pool_waits_total"))
}

func TestRequestMetrics(t *testing.T) {
	s := apitest.New(t)
	requests := func(labels ...string) float64 { return s.Metric(t, "goden_http_requests_total", labels...) }

	create(t, s, "Сливки 33%")
	require.EqualValues(t, 1, requests("operation", "CreateProduct", "status", "201"))
	require.EqualValues(t, 1, s.Metric(t, "goden_http_request_duration_seconds_count", "operation", "CreateProduct"))

	// отказ до обработчика операции тоже учтён и относится к своей операции
	resp, _ := raw(t, s, http.MethodGet, companyPath(s)+"/products", "", map[string]string{"Authorization": ""})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.EqualValues(t, 1, requests("operation", "ListProducts", "status", "401"))

	// идентификаторы из пути в метки не попадают: сто разных продуктов - одна серия
	for range 100 {
		_, err := s.Client.GetProduct(ctx, oas.GetProductParams{CompanyId: s.CompanyID, ProductId: uuid.Must(uuid.NewV7())})
		requireProblem(t, err, http.StatusNotFound)
	}
	require.Equal(t, 1, s.MetricSeries(t, "goden_http_requests_total", "operation", "GetProduct"))
	require.EqualValues(t, 100, requests("operation", "GetProduct", "status", "404"))

	resp, _ = raw(t, s, http.MethodGet, api.Prefix+"/no-such-path", "", nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 1, requests("operation", "unknown", "status", "404"))

	// служебные эндпоинты в метрики API не попадают
	before := requests()
	for _, path := range []string{"/metrics", "/healthz", "/readyz"} {
		resp, _ := raw(t, s, http.MethodGet, path, "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	require.Equal(t, before, requests())
}

func TestCommandMetricsThroughAPI(t *testing.T) {
	s := apitest.New(t)
	commands := func(name, outcome string) float64 {
		return s.Metric(t, "goden_commands_total", "command", name, "outcome", outcome)
	}
	params := oas.CreateProductParams{CompanyId: s.CompanyID, IdempotencyKey: oas.NewOptString("product-1")}

	_, err := s.Client.CreateProduct(ctx, newProduct("Сливки 33%"), params)
	require.NoError(t, err)
	_, err = s.Client.CreateProduct(ctx, newProduct("Сливки 33%"), params)
	require.NoError(t, err)
	require.EqualValues(t, 1, commands("CreateProduct", "ok"))
	require.EqualValues(t, 1, commands("CreateProduct", "replayed"))

	// то же название без ключа: правило предметной области
	_, err = s.Client.CreateProduct(ctx, newProduct("Сливки 33%"), oas.CreateProductParams{CompanyId: s.CompanyID})
	requireProblem(t, err, http.StatusConflict)
	require.EqualValues(t, 1, commands("CreateProduct", "error"))
	require.EqualValues(t, 1, s.Metric(t, "goden_events_appended_total", "type", "ProductCreated"))
}

func TestBusinessMetrics(t *testing.T) {
	s := apitest.New(t)
	events := func(eventType string) float64 { return s.Metric(t, "goden_events_appended_total", "type", eventType) }
	cream := create(t, s, "Сливки 33%")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 100)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	batchID := supply(t, s, cream, 250, oct(20))
	require.EqualValues(t, 1, events("DeliveryRegistered"))
	require.EqualValues(t, 1, events("DeliveryApproved"))

	// принятый заказ, его повтор с тем же ключом и отклонённый заказ
	params := oas.PlaceOrderParams{PointId: s.PointID, IdempotencyKey: oas.NewOptString("order-1")}
	for range 2 {
		placed, err := s.Client.PlaceOrder(ctx, order(portions(dish, 2)), params)
		require.NoError(t, err)
		require.Equal(t, oas.OrderStatusAccepted, placed.Status)
	}
	rejected, err := s.Client.PlaceOrder(ctx, order(portions(dish, 2)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, rejected.Status)
	require.EqualValues(t, 1, events("OrderAccepted"), "a repeated request with the same key is not a second order")
	require.EqualValues(t, 1, events("OrderRejected"))
	require.EqualValues(t, 1, events("StockConsumed"))

	// списание: количество в единице продукта
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 30, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
	require.NoError(t, err)
	unit := string(cream.Unit)
	require.EqualValues(t, 30, s.Metric(t, "goden_stock_written_off_quantity_total", "reason", "physical_damage", "unit", unit))
	require.EqualValues(t, 1, s.Metric(t, "goden_stock_write_offs_total", "reason", "physical_damage"))

	// отклонённое списание ничего не считает
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 1000, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "c",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
	requireProblem(t, err, http.StatusUnprocessableEntity)
	require.EqualValues(t, 30, s.Metric(t, "goden_stock_written_off_quantity_total"))
	require.EqualValues(t, 1, s.Metric(t, "goden_stock_write_offs_total"))
}
