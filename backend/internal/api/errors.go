package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ogen-go/ogen/ogenerrors"

	"goden/backend/internal/analytics"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/suppliers"
)

var internalProblem = httpx.Problem{
	Type: "internal-error", Title: "Внутренняя ошибка", Status: http.StatusInternalServerError,
}

// errBadPointID: идентификатор точки в адресе ручного маршрута не разобран.
var errBadPointID = errors.New("api: point id is not a uuid")

// errNoToken: запрос пришёл без токена.
var errNoToken = ogenerrors.ErrSecurityRequirementIsNotSatisfied

// invalidField реализуют ValidationError всех модулей.
type invalidField interface {
	error
	InvalidField() (field, message string)
}

// Sentinel-ошибки модулей по статусам. Новая ошибка модуля добавляется сюда,
// иначе она уйдёт клиенту как 500.
var (
	notFound = []error{catalog.ErrNotFound, inventory.ErrNotFound, orders.ErrNotFound, organization.ErrNotFound, suppliers.ErrNotFound}
	conflict = []error{
		catalog.ErrNameTaken,
		organization.ErrNameTaken, organization.ErrAlreadyAssigned, organization.ErrDisabled,
		suppliers.ErrNameTaken,
	}
	// конфликты, которые клиент обрабатывает по-разному, получают свой type
	namedConflict = []struct {
		err  error
		kind string
	}{
		{inventory.ErrAlreadyApproved, "delivery-already-approved"},
		{inventory.ErrNotApproved, "delivery-not-approved"},
		{inventory.ErrQuarantineClosed, "quarantine-closed"},
		{catalog.ErrUnitLocked, "unit-locked"},
		{orders.ErrNotCancellable, "order-not-cancellable"},
	}
)

// fieldDetail реализуют ошибки валидации, у которых клиенту нужен вид ошибки без разбора текста.
type fieldDetail interface {
	FieldDetail() (code string, limit *int64)
}

// statusClientClosedRequest - нестандартный статус для запроса, который клиент оборвал сам.
const statusClientClosedRequest = 499

// problemFor - единственное место, где ошибки превращаются в статусы HTTP:
// 401 - нет корректного токена, 403 - нет права на операцию, 404 - объект не найден,
// 409 - конфликт с текущим состоянием, 422 - нарушено правило предметной области,
// 500 - всё остальное, без текста исходной ошибки.
func problemFor(err error) httpx.Problem {
	var validation invalidField
	var forbidden *forbiddenError
	var unknownProduct *catalog.UnknownProductError
	var unknownDish *orders.UnknownDishError
	var noRecipe *orders.DishWithoutRecipeError
	var notSold *orders.DishNotSoldError
	var invalidPeriod *analytics.InvalidPeriodError
	var archivedProduct *catalog.ArchivedProductError
	var productInUse *catalog.ProductInUseError
	var archivedIngredients *catalog.ArchivedIngredientsError
	var invalidTimeZone *analytics.InvalidTimeZoneError

	switch {
	case isUnauthorized(err):
		// причина отказа клиенту не сообщается: она помогла бы подбирать токен
		return httpx.Problem{
			Type: "unauthorized", Title: "Требуется аутентификация", Status: http.StatusUnauthorized,
			Detail: "требуется действительный токен доступа",
		}
	case errors.As(err, &forbidden):
		detail := "операция недоступна"
		if forbidden.permission != "" {
			detail = fmt.Sprintf("требуется право %s", forbidden.permission)
		}
		return httpx.Problem{Type: "forbidden", Title: "Доступ запрещён", Status: http.StatusForbidden, Detail: detail}
	case errors.Is(err, organization.ErrNotOwner):
		return httpx.Problem{
			Type: "forbidden", Title: "Доступ запрещён", Status: http.StatusForbidden,
			Detail: "только владелец может передать компанию",
		}
	case errors.Is(err, organization.ErrForbidden):
		return httpx.Problem{
			Type: "forbidden", Title: "Доступ запрещён", Status: http.StatusForbidden,
			Detail: "только владелец может управлять управляющими",
		}
	case errors.Is(err, errBadPointID):
		return httpx.Problem{
			Type: "bad-request", Title: "Запрос не соответствует контракту API", Status: http.StatusBadRequest,
			Detail: "pointId должен быть UUID",
		}
	case errors.As(err, &invalidPeriod):
		return httpx.Problem{
			Type: "bad-request", Title: "Некорректный период", Status: http.StatusBadRequest,
			Detail: "начало периода должно быть раньше его окончания",
		}
	case errors.As(err, &invalidTimeZone):
		return httpx.Problem{
			Type: "bad-request", Title: "Некорректный часовой пояс", Status: http.StatusBadRequest,
			Detail: "неизвестный часовой пояс " + invalidTimeZone.Name,
		}
	case errors.As(err, &validation):
		field, message := validation.InvalidField()
		message = localizedValidationMessage(message)
		fieldError := httpx.FieldError{Field: field, Message: message}
		var detail fieldDetail
		if errors.As(err, &detail) {
			fieldError.Code, fieldError.Limit = detail.FieldDetail()
		}
		return httpx.Problem{
			Type: "validation-error", Title: "Ошибка валидации", Status: http.StatusUnprocessableEntity,
			Detail: fmt.Sprintf("некорректное значение поля %s: %s", field, message),
			Errors: []httpx.FieldError{fieldError},
		}
	case errors.As(err, &archivedProduct):
		return httpx.Problem{
			Type: "archived-reference", Title: "Продукт в архиве", Status: http.StatusUnprocessableEntity,
			Detail: fmt.Sprintf("продукт %q в архиве", archivedProduct.Name),
		}
	case errors.As(err, &productInUse):
		return httpx.Problem{
			Type: "product-in-use", Title: "Конфликт", Status: http.StatusConflict,
			Detail: "продукт есть в рецептурах блюд: " + strings.Join(productInUse.Dishes, ", "),
		}
	case errors.As(err, &archivedIngredients):
		return httpx.Problem{
			Type: "archived-ingredients", Title: "Конфликт", Status: http.StatusConflict,
			Detail: "в рецептуре есть продукты в архиве: " + strings.Join(archivedIngredients.Products, ", "),
		}
	case errors.As(err, &unknownProduct):
		return httpx.Problem{
			Type: "unknown-reference", Title: "Неизвестная ссылка", Status: http.StatusUnprocessableEntity,
			Detail: fmt.Sprintf("продукт %s не найден", unknownProduct.ProductID),
		}
	case errors.As(err, &unknownDish):
		return httpx.Problem{
			Type: "unknown-reference", Title: "Неизвестная ссылка", Status: http.StatusUnprocessableEntity,
			Detail: fmt.Sprintf("блюдо %s не найдено", unknownDish.DishID),
		}
	case errors.As(err, &noRecipe):
		return httpx.Problem{
			Type: "unknown-reference", Title: "Неизвестная ссылка", Status: http.StatusUnprocessableEntity,
			Detail: fmt.Sprintf("для блюда %s не задана рецептура", noRecipe.DishID),
		}
	case errors.As(err, &notSold):
		return httpx.Problem{
			Type: "unknown-reference", Title: "Неизвестная ссылка", Status: http.StatusUnprocessableEntity,
			Detail: fmt.Sprintf("блюдо %s не продаётся на этой точке", notSold.DishID),
		}
	case errors.Is(err, organization.ErrUnknownUser):
		const detail = "пользователь с таким адресом электронной почты не зарегистрирован"
		return httpx.Problem{
			Type: "unknown-reference", Title: "Неизвестная ссылка", Status: http.StatusUnprocessableEntity,
			Detail: detail,
			// поле нужно форме, чтобы показать ошибку у почты
			Errors: []httpx.FieldError{{Field: "email", Message: detail, Code: "unknown"}},
		}
	case errors.Is(err, command.ErrKeyReused):
		return httpx.Problem{
			Type: "idempotency-key-reused", Title: "Ключ идемпотентности использован повторно", Status: http.StatusUnprocessableEntity,
			Detail: "Idempotency-Key уже использован для другого запроса",
		}
	case isAny(err, notFound):
		return httpx.Problem{Type: "not-found", Title: "Объект не найден", Status: http.StatusNotFound, Detail: "запрошенный объект не найден"}
	// конфликт версий - подробности движка (имя команды, поток, версии) клиенту не нужны
	case errors.Is(err, eventstore.ErrVersionConflict):
		return httpx.Problem{
			Type: "version-conflict", Title: "Конфликт", Status: http.StatusConflict,
			Detail: "объект изменён параллельным запросом, повторите попытку",
		}
	case isAny(err, conflict):
		problem := httpx.Problem{Type: "conflict", Title: "Конфликт", Status: http.StatusConflict, Detail: conflictDetail(err)}
		if errors.Is(err, catalog.ErrNameTaken) || errors.Is(err, organization.ErrNameTaken) || errors.Is(err, suppliers.ErrNameTaken) {
			problem.Errors = []httpx.FieldError{nameTakenField(err)}
		}
		return problem
	// клиент разорвал соединение: это не внутренняя ошибка, в журнал ошибок и в 5xx метрик она не идёт
	case errors.Is(err, context.Canceled):
		return httpx.Problem{Type: "client-closed-request", Title: "Клиент закрыл запрос", Status: statusClientClosedRequest}
	default:
		for _, named := range namedConflict {
			if errors.Is(err, named.err) {
				return httpx.Problem{Type: named.kind, Title: "Конфликт", Status: http.StatusConflict, Detail: conflictDetail(err)}
			}
		}
		return internalProblem
	}
}

// nameTakenField - ошибка у поля name: занято работающим или архивным продуктом либо блюдом.
func nameTakenField(err error) httpx.FieldError {
	field := httpx.FieldError{Field: "name", Message: "название уже занято", Code: "taken"}
	var detail fieldDetail
	if errors.As(err, &detail) {
		field.Code, _ = detail.FieldDetail()
	}
	if field.Code == "taken_by_archived" {
		field.Message = "название занято продуктом или блюдом в архиве"
	}
	return field
}

func conflictDetail(err error) string {
	switch {
	case errors.Is(err, catalog.ErrNameTaken), errors.Is(err, organization.ErrNameTaken), errors.Is(err, suppliers.ErrNameTaken):
		return "название уже занято"
	case errors.Is(err, organization.ErrAlreadyAssigned):
		return "пользователь уже назначен на эту точку"
	case errors.Is(err, organization.ErrDisabled):
		return "компания или точка отключена"
	case errors.Is(err, inventory.ErrAlreadyApproved):
		return "поставка уже принята"
	case errors.Is(err, inventory.ErrNotApproved):
		return "поставка ещё не принята"
	case errors.Is(err, inventory.ErrQuarantineClosed):
		return "у позиции нет открытого карантина"
	case errors.Is(err, catalog.ErrUnitLocked):
		return "единицу продукта уже нельзя менять: он есть в рецептуре, поставке или на складе"
	case errors.Is(err, orders.ErrNotCancellable):
		return "отменить можно только принятый заказ: этот отклонён или уже отменён"
	default:
		return "операция конфликтует с текущим состоянием"
	}
}

func localizedValidationMessage(message string) string {
	exact := map[string]string{
		"must not be empty":         "не должно быть пустым",
		"must be positive":          "должно быть положительным",
		"must not be zero":          "не должно быть равно нулю",
		"is required":               "обязательно",
		"must be one of g, ml, pcs": "должно быть одним из значений: g, ml, pcs",
		"must be one of manager, approver, cook, waiter":   "должно быть одной из ролей: manager, approver, cook, waiter",
		"must not be after expires_at":                     "не должно быть позже expires_at",
		"quantities must not be negative":                  "количества не должны быть отрицательными",
		"item is expired and cannot be accepted":           "позиция просрочена и не может быть принята",
		"is required for rejected quantity":                "обязательно при наличии отклонённого количества",
		"is required for quarantined quantity":             "обязательно при наличии количества в карантине",
		"is required for rejected or quarantined quantity": "обязательно при наличии отклонённого количества или количества в карантине",
		"item is expired, accepted quantity cannot grow":   "позиция просрочена, принятое количество нельзя увеличить",
		"correction changes nothing":                       "исправление ничего не меняет",
		"unknown write-off reason":                         "неизвестная причина списания",
		"batch is not expired yet":                         "срок годности партии ещё не истёк",
		"must be one of pinned, hidden, auto":              "должно быть одним из значений: pinned, hidden, auto",
		"must be a known IANA time zone":                   "должно быть часовым поясом IANA, например Europe/Moscow",
		"must contain at least one point":                  "нужна хотя бы одна точка",
		"must be working points of the company":            "точки должны быть работающими точками компании",
	}
	if translated, ok := exact[message]; ok {
		return translated
	}

	replacements := []struct {
		prefix string
		suffix string
	}{
		{"must not exceed ", "не должно превышать "},
		{"quantity of product ", "количество продукта "},
		{"portions of dish ", "количество порций блюда "},
		{"product ", "продукт "},
		{"dish ", "блюдо "},
		{"item ", "позиция "},
		{"exceeds the product shelf life of ", "превышает срок хранения продукта: "},
		{"each quantity must not exceed declared quantity ", "каждое количество не должно превышать заявленное количество "},
		{"accepted + rejected + quarantined must equal declared quantity ", "сумма принятого, отклонённого и карантинного количества должна равняться заявленному количеству "},
		{"accepted + rejected must equal the quarantined quantity ", "сумма принятого и отклонённого количества должна равняться количеству в карантине "},
		{"can be reduced by at most ", "можно уменьшить не более чем на "},
		{"exceeds the batch quantity ", "превышает количество в партии "},
		{"batch quantity must not exceed ", "количество в партии не должно превышать "},
		{"must be between ", "должно быть в диапазоне "},
	}
	for _, replacement := range replacements {
		if rest, ok := strings.CutPrefix(message, replacement.prefix); ok {
			return replacement.suffix + localizedValidationTail(rest)
		}
	}
	return "некорректное значение"
}

func localizedValidationTail(tail string) string {
	replacer := strings.NewReplacer(
		" must be positive", " должно быть положительным",
		" must not exceed ", " не должно превышать ",
		" characters", " символов",
		" is listed twice", " указано дважды",
		" is decided twice", " обработана дважды",
		" has no decision", " не имеет решения",
		" is not part of the delivery", " не входит в поставку",
		" is corrected twice", " исправлена дважды",
		": the rest is already consumed", ": остальное количество уже израсходовано",
		" and ", " и ",
		" days", " дней",
	)
	return replacer.Replace(tail)
}

func isAny(err error, targets []error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// problemResponse переводит ошибку в тип сгенерированного ответа и добавляет correlation ID запроса.
func problemResponse(ctx context.Context, p httpx.Problem) oas.Problem {
	out := oas.Problem{Type: p.Type, Title: p.Title, Status: int32(p.Status)}
	if p.Detail != "" {
		out.Detail = oas.NewOptString(p.Detail)
	}
	if id, ok := observability.CorrelationID(ctx); ok {
		out.CorrelationID = oas.NewOptUUID(id)
	}
	for _, fieldError := range p.Errors {
		item := oas.FieldError{Field: fieldError.Field, Message: fieldError.Message}
		if fieldError.Code != "" {
			item.Code = oas.NewOptString(fieldError.Code)
		}
		if fieldError.Limit != nil {
			item.Limit = oas.NewOptInt64(*fieldError.Limit)
		}
		out.Errors = append(out.Errors, item)
	}
	return out
}
