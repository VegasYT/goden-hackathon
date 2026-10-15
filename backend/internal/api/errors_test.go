package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/analytics"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
)

func TestProblemFor(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	wrap := func(err error) error { return fmt.Errorf("product %s: %w", id, err) }

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
		wantTitle  string
		wantDetail string
	}{
		{"unauthorized", errNoToken, http.StatusUnauthorized, "unauthorized", "Требуется аутентификация", "требуется действительный токен доступа"},
		{"forbidden by permission", &forbiddenError{permission: "orders.accept"}, http.StatusForbidden, "forbidden", "Доступ запрещён", "требуется право orders.accept"},
		{"forbidden by domain rule", organization.ErrForbidden, http.StatusForbidden, "forbidden", "Доступ запрещён", "только владелец может управлять управляющими"},
		{"transfer by a former owner", fmt.Errorf("company: %w", organization.ErrNotOwner), http.StatusForbidden, "forbidden", "Доступ запрещён", "только владелец может передать компанию"},
		{"bad point id", errBadPointID, http.StatusBadRequest, "bad-request", "Запрос не соответствует контракту API", "pointId должен быть UUID"},
		{"invalid period", &analytics.InvalidPeriodError{From: time.Now(), To: time.Now()}, http.StatusBadRequest, "bad-request", "Некорректный период", "начало периода должно быть раньше его окончания"},
		{"catalog validation", &catalog.ValidationError{Field: "name", Message: "must not be empty"}, http.StatusUnprocessableEntity, "validation-error", "Ошибка валидации", "некорректное значение поля name: не должно быть пустым"},
		{"inventory validation", wrap(&inventory.ValidationError{Field: "quantity", Message: "must be positive"}), http.StatusUnprocessableEntity, "validation-error", "Ошибка валидации", "некорректное значение поля quantity: должно быть положительным"},
		{"orders validation", &orders.ValidationError{Field: "items", Message: "must not be empty"}, http.StatusUnprocessableEntity, "validation-error", "Ошибка валидации", "некорректное значение поля items: не должно быть пустым"},
		{"unknown product", &catalog.UnknownProductError{ProductID: id}, http.StatusUnprocessableEntity, "unknown-reference", "Неизвестная ссылка", fmt.Sprintf("продукт %s не найден", id)},
		{"unknown dish", &orders.UnknownDishError{DishID: id}, http.StatusUnprocessableEntity, "unknown-reference", "Неизвестная ссылка", fmt.Sprintf("блюдо %s не найдено", id)},
		{"dish without recipe", &orders.DishWithoutRecipeError{DishID: id}, http.StatusUnprocessableEntity, "unknown-reference", "Неизвестная ссылка", fmt.Sprintf("для блюда %s не задана рецептура", id)},
		{"dish not sold", &orders.DishNotSoldError{DishID: id}, http.StatusUnprocessableEntity, "unknown-reference", "Неизвестная ссылка", fmt.Sprintf("блюдо %s не продаётся на этой точке", id)},
		{"unknown user", organization.ErrUnknownUser, http.StatusUnprocessableEntity, "unknown-reference", "Неизвестная ссылка", "пользователь с таким адресом электронной почты не зарегистрирован"},
		{"idempotency key reused", wrap(command.ErrKeyReused), http.StatusUnprocessableEntity, "idempotency-key-reused", "Ключ идемпотентности использован повторно", "Idempotency-Key уже использован для другого запроса"},
		{"catalog not found", wrap(catalog.ErrNotFound), http.StatusNotFound, "not-found", "Объект не найден", "запрошенный объект не найден"},
		{"inventory not found", wrap(inventory.ErrNotFound), http.StatusNotFound, "not-found", "Объект не найден", "запрошенный объект не найден"},
		{"orders not found", wrap(orders.ErrNotFound), http.StatusNotFound, "not-found", "Объект не найден", "запрошенный объект не найден"},
		{"name taken", wrap(catalog.ErrNameTaken), http.StatusConflict, "conflict", "Конфликт", "название уже занято"},
		{"already approved", wrap(inventory.ErrAlreadyApproved), http.StatusConflict, "delivery-already-approved", "Конфликт", "поставка уже принята"},
		{"not approved", wrap(inventory.ErrNotApproved), http.StatusConflict, "delivery-not-approved", "Конфликт", "поставка ещё не принята"},
		{"quarantine closed", wrap(inventory.ErrQuarantineClosed), http.StatusConflict, "quarantine-closed", "Конфликт", "у позиции нет открытого карантина"},
		{"version conflict", wrap(eventstore.ErrVersionConflict), http.StatusConflict, "version-conflict", "Конфликт", "объект изменён параллельным запросом, повторите попытку"},
		{"client closed the connection", wrap(context.Canceled), 499, "client-closed-request", "Клиент закрыл запрос", ""},
		{"anything else", errors.New("pq: password authentication failed for user admin"), http.StatusInternalServerError, "internal-error", "Внутренняя ошибка", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem := problemFor(tt.err)

			require.Equal(t, tt.wantStatus, problem.Status)
			require.Equal(t, tt.wantType, problem.Type)
			require.Equal(t, tt.wantTitle, problem.Title)
			require.Equal(t, tt.wantDetail, problem.Detail)
		})
	}
}

// Вид ошибки поля и предел доходят до ответа вместе с переведённым сообщением.
func TestProblemForValidationCarriesCodeAndLimit(t *testing.T) {
	limit := int64(85)
	problem := problemFor(&inventory.ValidationError{
		Field: "items[1].accepted_quantity", Code: inventory.CodeStockConsumed, Limit: &limit,
		Message: "can be reduced by at most 85: the rest is already consumed",
	})

	require.Equal(t, []httpx.FieldError{{
		Field: "items[1].accepted_quantity", Code: "stock-consumed", Limit: &limit,
		Message: "можно уменьшить не более чем на 85: остальное количество уже израсходовано",
	}}, problem.Errors)
}

func TestProblemForValidationListsField(t *testing.T) {
	problem := problemFor(&catalog.ValidationError{Field: "name", Message: "must not be empty"})

	require.Len(t, problem.Errors, 1)
	require.Equal(t, "name", problem.Errors[0].Field)
	require.Equal(t, "не должно быть пустым", problem.Errors[0].Message)
}

func TestLocalizedValidationMessage(t *testing.T) {
	id := uuid.Must(uuid.NewV7()).String()
	tests := map[string]string{
		"must not be empty":         "не должно быть пустым",
		"must be positive":          "должно быть положительным",
		"must not be zero":          "не должно быть равно нулю",
		"is required":               "обязательно",
		"must be one of g, ml, pcs": "должно быть одним из значений: g, ml, pcs",
		"must be one of manager, approver, cook, waiter":                    "должно быть одной из ролей: manager, approver, cook, waiter",
		"must not be after expires_at":                                      "не должно быть позже expires_at",
		"must not exceed 3650":                                              "не должно превышать 3650",
		"quantity of product " + id + " must be positive":                   "количество продукта " + id + " должно быть положительным",
		"quantity of product " + id + " must not exceed 100":                "количество продукта " + id + " не должно превышать 100",
		"product " + id + " is listed twice":                                "продукт " + id + " указано дважды",
		"portions of dish " + id + " must be positive":                      "количество порций блюда " + id + " должно быть положительным",
		"dish " + id + " is listed twice":                                   "блюдо " + id + " указано дважды",
		"exceeds the product shelf life of 10 days":                         "превышает срок хранения продукта: 10 дней",
		"item " + id + " is decided twice":                                  "позиция " + id + " обработана дважды",
		"item " + id + " has no decision":                                   "позиция " + id + " не имеет решения",
		"item " + id + " is not part of the delivery":                       "позиция " + id + " не входит в поставку",
		"item " + id + " is corrected twice":                                "позиция " + id + " исправлена дважды",
		"quantities must not be negative":                                   "количества не должны быть отрицательными",
		"each quantity must not exceed declared quantity 20":                "каждое количество не должно превышать заявленное количество 20",
		"accepted + rejected + quarantined must equal declared quantity 20": "сумма принятого, отклонённого и карантинного количества должна равняться заявленному количеству 20",
		"accepted + rejected must equal the quarantined quantity 20":        "сумма принятого и отклонённого количества должна равняться количеству в карантине 20",
		"item is expired and cannot be accepted":                            "позиция просрочена и не может быть принята",
		"is required for rejected quantity":                                 "обязательно при наличии отклонённого количества",
		"is required for quarantined quantity":                              "обязательно при наличии количества в карантине",
		"is required for rejected or quarantined quantity":                  "обязательно при наличии отклонённого количества или количества в карантине",
		"item is expired, accepted quantity cannot grow":                    "позиция просрочена, принятое количество нельзя увеличить",
		"can be reduced by at most 5: the rest is already consumed":         "можно уменьшить не более чем на 5: остальное количество уже израсходовано",
		"correction changes nothing":                                        "исправление ничего не меняет",
		"exceeds the batch quantity 7":                                      "превышает количество в партии 7",
		"unknown write-off reason":                                          "неизвестная причина списания",
		"batch is not expired yet":                                          "срок годности партии ещё не истёк",
		"must be between -100 and 100":                                      "должно быть в диапазоне -100 и 100",
		"batch quantity must not exceed 100":                                "количество в партии не должно превышать 100",
		"new validation text":                                               "некорректное значение",
	}
	for message, want := range tests {
		t.Run(message, func(t *testing.T) {
			require.Equal(t, want, localizedValidationMessage(message))
		})
	}
}

func TestRequestErrorHandlerLocalizesContractError(t *testing.T) {
	w := httptest.NewRecorder()
	err := &ogenerrors.DecodeParamsError{
		OperationContext: ogenerrors.OperationContext{Name: "ListProducts", ID: "listProducts"},
		Err:              &ogenerrors.DecodeParamError{Name: "limit", In: "query", Err: errors.New("invalid value")},
	}

	requestErrorHandler(slog.New(slog.DiscardHandler))(context.Background(), w, nil, err)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var problem httpx.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
	require.Equal(t, "bad-request", problem.Type)
	require.Equal(t, "Запрос не соответствует контракту API", problem.Title)
	require.Equal(t, "проверьте параметры и тело запроса", problem.Detail)
}

func TestRoutingErrorsAreLocalized(t *testing.T) {
	tests := []struct {
		name        string
		handle      func(http.ResponseWriter, *http.Request)
		status      int
		problemType string
		title       string
		detail      string
	}{
		{
			name:   "not found",
			handle: notFoundHandler,
			status: http.StatusNotFound, problemType: "not-found",
			title: "Путь не найден", detail: "путь не существует: /api/v1/missing",
		},
		{
			name: "method not allowed",
			handle: func(w http.ResponseWriter, r *http.Request) {
				methodNotAllowedHandler(w, r, http.MethodGet)
			},
			status: http.StatusMethodNotAllowed, problemType: "method-not-allowed",
			title: "Метод не разрешён", detail: "HTTP-метод не поддерживается для этого пути",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.handle(w, httptest.NewRequest(http.MethodPost, "/api/v1/missing", nil))

			require.Equal(t, tt.status, w.Code)
			var problem httpx.Problem
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
			require.Equal(t, tt.problemType, problem.Type)
			require.Equal(t, tt.title, problem.Title)
			require.Equal(t, tt.detail, problem.Detail)
		})
	}
}

func TestProblemForInternalErrorHidesDetails(t *testing.T) {
	problem := problemFor(errors.New("pq: password authentication failed for user admin"))

	require.Empty(t, problem.Detail)
	require.NotContains(t, problem.Title, "password")
}

// Конфликт версий после исчерпания повторов не раскрывает имя команды, поток и версии.
func TestProblemForVersionConflictHidesEngineDetails(t *testing.T) {
	err := fmt.Errorf(`command PlaceOrder: gave up after 3 attempts: stream "stock-0199": expected version 5, actual 6: %w`,
		eventstore.ErrVersionConflict)

	problem := problemFor(err)

	require.Equal(t, http.StatusConflict, problem.Status)
	require.NotContains(t, problem.Detail, "PlaceOrder")
	require.NotContains(t, problem.Detail, "stock-")
	require.NotContains(t, problem.Detail, "version 5")
}
