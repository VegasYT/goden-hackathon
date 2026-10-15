//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
)

var ctx = context.Background()

func newProduct(name string) *oas.NewProduct {
	return &oas.NewProduct{Name: name, Unit: oas.UnitG, Category: oas.NewOptString("Тест")}
}

func create(t *testing.T, s apitest.Server, name string) *oas.Product {
	t.Helper()
	product, err := s.Client.CreateProduct(ctx, newProduct(name), oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	return plain(product)
}

// plain - продукт без полей, которые есть только в ответах каталога: так его показывают
// склад, заказы, рекомендации и аналитика.
func plain(product *oas.Product) *oas.Product {
	out := *product
	out.Dishes, out.UnitLocked, out.UsedIn = oas.OptInt64{}, oas.OptBool{}, nil
	return &out
}

// requireProblem проверяет, что клиент получил ошибку API с ожидаемым статусом.
// pointPath - начало адресов операций точки пользователя dev.
func pointPath(s apitest.Server) string {
	return api.Prefix + "/points/" + s.PointID.String()
}

// companyPath - начало адресов каталога компании пользователя dev.
func companyPath(s apitest.Server) string {
	return api.Prefix + "/companies/" + s.CompanyID.String()
}

func requireProblem(t *testing.T, err error, status int) oas.Problem {
	t.Helper()
	var apiErr *oas.ErrorStatusCode
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, status, apiErr.StatusCode)
	require.EqualValues(t, status, apiErr.Response.Status)
	require.NotEmpty(t, apiErr.Response.Type)
	require.NotEmpty(t, apiErr.Response.Title)
	require.True(t, apiErr.Response.CorrelationID.Set, "an error carries the correlation id")
	return apiErr.Response
}

// raw отправляет запрос в обход клиента: то, что по контракту отправить нельзя.
func raw(t *testing.T, s apitest.Server, method, path, body string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, s.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	for name, value := range headers {
		if value == "" {
			req.Header.Del(name) // пустое значение убирает заголовок, в том числе токен по умолчанию
			continue
		}
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, data
}

func requireRawProblem(t *testing.T, resp *http.Response, body []byte, status int) httpx.Problem {
	t.Helper()
	require.Equal(t, status, resp.StatusCode, string(body))
	require.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	var problem httpx.Problem
	require.NoError(t, json.Unmarshal(body, &problem))
	require.Equal(t, status, problem.Status)
	require.NotEmpty(t, problem.Type)
	require.NotEmpty(t, problem.Title)
	require.NotNil(t, problem.CorrelationID)
	return problem
}

// Продукты.

func TestCreateAndGetProduct(t *testing.T) {
	s := apitest.New(t)

	created, err := s.Client.CreateProduct(ctx, &oas.NewProduct{
		Name: "Сливки 33%", Unit: oas.UnitMl, Category: oas.NewOptString("Молочные продукты"), ShelfLifeDays: oas.NewOptInt32(5),
	}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)

	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, "Сливки 33%", created.Name)
	require.Equal(t, oas.UnitMl, created.Unit)
	require.Equal(t, "Молочные продукты", created.Category)
	require.Equal(t, oas.NewOptInt32(5), created.ShelfLifeDays)

	read, err := s.Client.GetProduct(ctx, oas.GetProductParams{CompanyId: s.CompanyID, ProductId: created.ID})
	require.NoError(t, err)
	require.Empty(t, read.UsedIn, "a new product is in no recipe")
	read.UsedIn = nil
	require.Equal(t, created, read)
}

func TestCreateProductReturns201(t *testing.T) {
	s := apitest.New(t)

	resp, body := raw(t, s, http.MethodPost, companyPath(s)+"/products", `{"name":"Соль","unit":"g"}`, nil)

	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	var product struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	require.NoError(t, json.Unmarshal(body, &product))
	require.Equal(t, "Соль", product.Name)
}

func TestListProducts(t *testing.T) {
	s := apitest.New(t)
	for _, name := range []string{"Яйцо", "Бекон", "Сливки 33%"} {
		create(t, s, name)
	}

	page, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, int64(3), page.Total)
	names := make([]string, len(page.Items))
	for i, item := range page.Items {
		names[i] = item.Name
	}
	require.Equal(t, []string{"Бекон", "Сливки 33%", "Яйцо"}, names)

	tail, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID, Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(2)})
	require.NoError(t, err)
	require.Equal(t, int64(3), tail.Total)
	require.Len(t, tail.Items, 1)
	require.Equal(t, "Яйцо", tail.Items[0].Name)
}

func TestListProductsPagination(t *testing.T) {
	s := apitest.New(t)
	for i := range 120 {
		create(t, s, fmt.Sprintf("Продукт %03d", i))
	}

	first, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Len(t, first.Items, 50, "default page size")
	require.Equal(t, int64(120), first.Total)

	last, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID, Limit: oas.NewOptInt32(50), Offset: oas.NewOptInt32(100)})
	require.NoError(t, err)
	require.Len(t, last.Items, 20)
	require.Equal(t, int64(120), last.Total)

	for _, query := range []string{"limit=1000", "limit=0", "offset=-1", "limit=abc"} {
		resp, body := raw(t, s, http.MethodGet, companyPath(s)+"/products?"+query, "", nil)
		requireRawProblem(t, resp, body, http.StatusBadRequest)
	}
}

// Ошибки.

func TestValidationError(t *testing.T) {
	s := apitest.New(t)

	_, err := s.Client.CreateProduct(ctx, newProduct("   "), oas.CreateProductParams{CompanyId: s.CompanyID})

	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "validation-error", problem.Type)
	require.Equal(t, []oas.FieldError{{Field: "name", Message: "не должно быть пустым"}}, problem.Errors)
}

func TestNotFound(t *testing.T) {
	s := apitest.New(t)

	_, err := s.Client.GetProduct(ctx, oas.GetProductParams{CompanyId: s.CompanyID, ProductId: uuid.Must(uuid.NewV7())})

	problem := requireProblem(t, err, http.StatusNotFound)
	require.Equal(t, "not-found", problem.Type)
}

func TestConflict(t *testing.T) {
	s := apitest.New(t)
	create(t, s, "Сливки 33%")

	_, err := s.Client.CreateProduct(ctx, newProduct(" сливки 33% "), oas.CreateProductParams{CompanyId: s.CompanyID})

	problem := requireProblem(t, err, http.StatusConflict)
	require.Equal(t, "conflict", problem.Type)
}

func TestRequestsThatDoNotMatchTheContract(t *testing.T) {
	s := apitest.New(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{"missing required field", http.MethodPost, "/products", `{"name":"Мука"}`, http.StatusBadRequest},
		{"unknown unit", http.MethodPost, "/products", `{"name":"Мука","unit":"kg"}`, http.StatusBadRequest},
		{"non-positive shelf life", http.MethodPost, "/products", `{"name":"Мука","unit":"g","shelf_life_days":0}`, http.StatusBadRequest},
		{"broken json", http.MethodPost, "/products", `{"name":`, http.StatusBadRequest},
		// название в кодировке Windows-1251: без проверки оно дошло бы до базы и дало 500
		{"body is not utf-8", http.MethodPost, "/products", "{\"name\":\"\xd1\xeb\xe8\xe2\xea\xe8\",\"unit\":\"g\"}", http.StatusBadRequest},
		{"id is not a uuid", http.MethodGet, "/products/42", "", http.StatusBadRequest},
		{"unknown path", http.MethodGet, "/no-such-thing", "", http.StatusNotFound},
		{"method not allowed", http.MethodDelete, "/products", "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := raw(t, s, tt.method, companyPath(s)+tt.path, tt.body, nil)
			requireRawProblem(t, resp, body, tt.status)
		})
	}

	// ни один из запросов не дошёл до бизнес-логики
	page, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Zero(t, page.Total)
}

// Идемпотентность.

func TestIdempotencyKey(t *testing.T) {
	s := apitest.New(t)
	params := oas.CreateProductParams{CompanyId: s.CompanyID, IdempotencyKey: oas.NewOptString("key-1")}

	first, err := s.Client.CreateProduct(ctx, newProduct("Бекон"), params)
	require.NoError(t, err)
	second, err := s.Client.CreateProduct(ctx, newProduct("Бекон"), params)
	require.NoError(t, err)

	require.Equal(t, first, second)
	page, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)

	// тот же ключ с другим телом
	_, err = s.Client.CreateProduct(ctx, newProduct("Ветчина"), params)
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "idempotency-key-reused", problem.Type)

	// ключ с недопустимыми символами отклоняется контрактом
	resp, body := raw(t, s, http.MethodPost, companyPath(s)+"/products", `{"name":"Сыр","unit":"g"}`,
		map[string]string{"Idempotency-Key": "ключ с пробелами"})
	requireRawProblem(t, resp, body, http.StatusBadRequest)
}

// Correlation ID.

func TestCorrelationIDReachesTheEvent(t *testing.T) {
	s := apitest.New(t)
	correlationID := uuid.Must(uuid.NewV7())

	resp, body := raw(t, s, http.MethodPost, companyPath(s)+"/products", `{"name":"Бекон","unit":"g"}`,
		map[string]string{httpx.CorrelationHeader: correlationID.String()})
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
	require.Equal(t, correlationID.String(), resp.Header.Get(httpx.CorrelationHeader))

	var product struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &product))
	events, err := eventstore.ReadStream(ctx, s.Pool, catalog.ProductStream(product.ID), 1)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, correlationID, events[0].CorrelationID)
	require.JSONEq(t, `{"actor":{"type":"user","id":"dev"},"command":"CreateProduct"}`, string(events[0].Metadata))
}

func TestCorrelationIDIsCreatedByTheServer(t *testing.T) {
	s := apitest.New(t)

	resp, _ := raw(t, s, http.MethodGet, companyPath(s)+"/products", "", nil)

	_, err := uuid.Parse(resp.Header.Get(httpx.CorrelationHeader))
	require.NoError(t, err)

	// correlation ID из заголовка ответа совпадает с тем, что в теле ошибки
	resp, body := raw(t, s, http.MethodGet, companyPath(s)+"/products/"+uuid.Must(uuid.NewV7()).String(), "", nil)
	problem := requireRawProblem(t, resp, body, http.StatusNotFound)
	require.Equal(t, resp.Header.Get(httpx.CorrelationHeader), problem.CorrelationID.String())
}

// Health.

func TestHealthEndpoints(t *testing.T) {
	s := apitest.New(t)

	for _, path := range []string{"/healthz", "/readyz"} {
		resp, _ := raw(t, s, http.MethodGet, path, "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
	}

	s.Pool.Close() // база стала недоступна

	resp, _ := raw(t, s, http.MethodGet, "/healthz", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, _ = raw(t, s, http.MethodGet, "/readyz", "", nil)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// Сбой внутри обработчика не раскрывает подробности клиенту.
func TestInternalErrorHidesDetails(t *testing.T) {
	s := apitest.New(t)
	s.Pool.Close()

	_, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})

	problem := requireProblem(t, err, http.StatusInternalServerError)
	require.Equal(t, "internal-error", problem.Type)
	require.False(t, problem.Detail.Set, "no details of the failure in the response")
	require.False(t, errors.Is(err, context.Canceled))
}
