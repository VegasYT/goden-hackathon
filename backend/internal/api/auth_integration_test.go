//go:build integration

package api_test

import (
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
)

// Все права из ADR-006.
var allPermissions = []string{
	"catalog.view", "deliveries.view", "deliveries.register", "deliveries.approve",
	"orders.view", "orders.accept", "orders.cancel", "inventory.view", "inventory.edit", "kitchen.view",
	"products.manage", "dishes.manage", "analytics.view", "recommendations.view", "generator.manage", "generator.run",
	"migom.integration", "supplier.access",
}

func user(id string) eventstore.Actor {
	return eventstore.Actor{Type: eventstore.ActorUser, ID: id}
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// Матрица прав строится по контракту: если в нём появится операция, которой нет
// в operations, она осталась бы без проверки доступа.
func TestEveryOperationHasARequest(t *testing.T) {
	contract, err := os.ReadFile("../../api/openapi.yaml")
	require.NoError(t, err)
	var declared []string
	for _, match := range regexp.MustCompile(`(?m)^\s+operationId: (\w+)`).FindAllStringSubmatch(string(contract), -1) {
		declared = append(declared, strings.ToUpper(match[1][:1])+match[1][1:])
	}
	require.NotEmpty(t, declared)

	var covered []string
	for _, op := range operations("id") {
		covered = append(covered, op.name)
	}
	require.ElementsMatch(t, declared, covered)
}

// Права, которые определяет не токен, а владение компанией и назначение на точку (ADR-013).
var organizationPermissions = []string{"company.view", "company.manage", "point.view", "staff.view", "staff.manage"}

func TestPermissionMatrix(t *testing.T) {
	s := apitest.New(t)
	nobody := s.TokenFor(user("nobody"))

	// чужая компания с точкой: на них проверяются операции с точкой и компанией в адресе
	owner := s.Login(t, "matrix-owner")
	company, err := owner.CreateCompany(ctx, &oas.NewCompany{Name: "Чужая"}, oas.CreateCompanyParams{})
	require.NoError(t, err)
	point, err := owner.CreatePoint(ctx, &oas.NewPoint{Name: "Точка"}, oas.CreatePointParams{CompanyId: company.ID})
	require.NoError(t, err)
	inCompany, onPoint := operations(company.ID.String()), operations(point.ID.String())

	for i, op := range operations(uuid.Must(uuid.NewV7()).String()) {
		t.Run(op.name, func(t *testing.T) {
			required := oas.GetRolesForBearerAuth(op.name)
			before := countEvents(t, s)

			resp, body := raw(t, s, op.method, api.Prefix+op.path, op.body, map[string]string{"Authorization": ""})
			problem := requireRawProblem(t, resp, body, http.StatusUnauthorized)
			require.Equal(t, "unauthorized", problem.Type)
			require.Equal(t, "Bearer", resp.Header.Get("WWW-Authenticate"))

			resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, bearer("not-a-token"))
			requireRawProblem(t, resp, body, http.StatusUnauthorized)

			if len(required) == 0 {
				// Эти операции не раскрывают бизнес-данные и доступны любому аутентифицированному клиенту.
				require.Contains(t, []string{
					oas.GetMeOperation, oas.CreateCompanyOperation,
				}, op.name)
				resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, bearer(nobody))
				require.Contains(t, []int{http.StatusOK, http.StatusCreated}, resp.StatusCode, string(body))
				return
			}
			require.Len(t, required, 1)
			if strings.HasPrefix(op.path, "/companies/") || strings.HasPrefix(op.path, "/points/") {
				// права в токене не помогают: у клиента нет отношения к этой точке и компании
				scoped := inCompany[i]
				if strings.HasPrefix(op.path, "/points/") {
					scoped = onPoint[i]
				}
				everything := s.TokenFor(user("everything"), append(slices.Clone(allPermissions), organizationPermissions...)...)
				resp, body = raw(t, s, scoped.method, api.Prefix+scoped.path, scoped.body, bearer(everything))
				problem = requireRawProblem(t, resp, body, http.StatusForbidden)
				require.Contains(t, problem.Detail, required[0], "the response names the missing permission")
				require.Equal(t, before, countEvents(t, s), "denied requests leave no events")

				// точки или компании нет вовсе
				resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, bearer(everything))
				requireRawProblem(t, resp, body, http.StatusNotFound)
				return
			}
			require.Contains(t, allPermissions, required[0])

			resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, bearer(nobody))
			problem = requireRawProblem(t, resp, body, http.StatusForbidden)
			require.Equal(t, "forbidden", problem.Type)
			require.Contains(t, problem.Detail, required[0], "the response names the missing permission")

			others := slices.DeleteFunc(slices.Clone(allPermissions), func(p string) bool { return p == required[0] })
			resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, bearer(s.TokenFor(user("others"), others...)))
			requireRawProblem(t, resp, body, http.StatusForbidden)
			require.Equal(t, before, countEvents(t, s), "denied requests leave no events")

			resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, bearer(s.TokenFor(user("exact"), required[0])))
			require.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode, string(body))
		})
	}
}

func TestHealthNeedsNoToken(t *testing.T) {
	s := apitest.New(t)
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, body := raw(t, s, http.MethodGet, path, "", map[string]string{"Authorization": ""})
		require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	}
}

func countEvents(t *testing.T, s apitest.Server) int {
	t.Helper()
	var n int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM events`).Scan(&n))
	return n
}

// Автор критичного действия берётся из токена и остаётся в событии вместе
// с идентификатором запроса: Event Store служит журналом аудита.
func TestActorComesFromTheToken(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 100, oct(15)))

	approver := eventstore.Actor{Type: eventstore.ActorUser, ID: "approver"}
	approved, err := s.Staff(t, approver.ID, oas.RoleApprover).
		ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "approver", Name: oas.NewOptString("approver")}, approved.ApprovedBy.Value)

	// списание от имени сервисной учётной записи и с заданным correlation ID
	correlationID := uuid.Must(uuid.NewV7())
	service := eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"}
	headers := bearer(s.Hire(t, service, oas.RoleManager))
	headers[httpx.CorrelationHeader] = correlationID.String()
	resp, body := raw(t, s, http.MethodPost, pointPath(s)+"/batches/"+approved.Items[0].BatchIds[0].String()+"/write-offs",
		`{"quantity":10,"reason":"other","comment":"c"}`, headers)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	type audit struct {
		actorType, actorID string
		correlationID      *uuid.UUID
	}
	read := func(eventType string) audit {
		var a audit
		require.NoError(t, s.Pool.QueryRow(ctx,
			`SELECT metadata->'actor'->>'type', metadata->'actor'->>'id', correlation_id
			 FROM events WHERE event_type = $1`, eventType).Scan(&a.actorType, &a.actorID, &a.correlationID))
		return a
	}
	approval := read("DeliveryApproved")
	require.Equal(t, "user", approval.actorType)
	require.Equal(t, "approver", approval.actorID)
	require.NotNil(t, approval.correlationID)

	writeOff := read("StockWrittenOff")
	require.Equal(t, "service", writeOff.actorType)
	require.Equal(t, "goden-generator", writeOff.actorID)
	require.Equal(t, correlationID, *writeOff.correlationID)
}

func TestDeniedRequestChangesNothing(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 100, oct(15)))
	// назначение повара - тоже событие, поэтому оно создаётся до подсчёта
	cook := s.Staff(t, "cook", oas.RoleCook)
	before := countEvents(t, s)

	_, err := cook.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	problem := requireProblem(t, err, http.StatusForbidden)
	require.Contains(t, problem.Detail.Value, "deliveries.approve")

	require.Equal(t, before, countEvents(t, s))
	delivery, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, oas.DeliveryStatusRegistered, delivery.Status)

	// свои операции повару доступны
	_, err = cook.GetMenu(ctx, oas.GetMenuParams{PointId: s.PointID})
	require.NoError(t, err)
}

func TestCORSThroughAPI(t *testing.T) {
	s := apitest.New(t)

	preflight := map[string]string{
		"Origin": apitest.Origin, "Access-Control-Request-Method": http.MethodPost,
		"Access-Control-Request-Headers": "authorization", "Authorization": "",
	}
	resp, _ := raw(t, s, http.MethodOptions, companyPath(s)+"/products", "", preflight)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "preflight needs no token")
	require.Equal(t, apitest.Origin, resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")

	resp, _ = raw(t, s, http.MethodGet, companyPath(s)+"/products", "", map[string]string{"Origin": apitest.Origin})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, apitest.Origin, resp.Header.Get("Access-Control-Allow-Origin"))

	// отказ тоже должен быть читаем страницей: иначе frontend не увидит 401
	resp, _ = raw(t, s, http.MethodGet, companyPath(s)+"/products", "", map[string]string{"Origin": apitest.Origin, "Authorization": ""})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, apitest.Origin, resp.Header.Get("Access-Control-Allow-Origin"))

	resp, _ = raw(t, s, http.MethodGet, companyPath(s)+"/products", "", map[string]string{"Origin": "http://evil.example"})
	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}
