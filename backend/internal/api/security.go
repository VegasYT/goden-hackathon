package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/ogen-go/ogen/middleware"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/generator"
	"goden/backend/internal/migom"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/realtime"
	"goden/backend/internal/suppliers"
)

// Операции, доступные любому аутентифицированному клиенту. Право остальных операций
// записано в контракте: security: bearerAuth: [<право>].
var authenticatedOnly = map[oas.OperationName]bool{
	// профиль и создание компании доступны любому вошедшему пользователю (ADR-013)
	oas.GetMeOperation:         true,
	oas.CreateCompanyOperation: true,
}

// forbiddenError - клиент аутентифицирован, но права на операцию у него нет.
type forbiddenError struct {
	actor      eventstore.Actor
	permission string
}

func (e *forbiddenError) Error() string {
	if e.permission == "" {
		return "the operation declares no permission"
	}
	return fmt.Sprintf("permission %s is required", e.permission)
}

// deniedError добавляет к отказу в доступе операцию и автора: по ним отказ находят в журнале.
// Ответ клиенту определяет вложенная ошибка.
type deniedError struct {
	operation string
	actor     eventstore.Actor
	err       error
}

func (e *deniedError) Error() string { return e.err.Error() }
func (e *deniedError) Unwrap() error { return e.err }

// security проверяет токен. Реализует oas.SecurityHandler. Право на операцию сверяет authorize:
// ему нужны параметры запроса, а они разбираются после проверки токена.
type security struct {
	verifier auth.Verifier
}

type principalKey struct{}

// HandleBearerAuth вызывается перед каждой операцией. t.Roles - права операции из контракта.
// Автор и личность из токена попадают в контекст, от имени автора выполняются команды.
func (s security) HandleBearerAuth(ctx context.Context, operation oas.OperationName, t oas.BearerAuth) (context.Context, error) {
	principal, err := s.verifier.Verify(ctx, t.Token)
	if err != nil {
		return nil, err
	}
	// операция без права в контракте закрыта для всех: забытое требование не открывает доступ
	if len(t.Roles) == 0 && !authenticatedOnly[operation] {
		return nil, &forbiddenError{actor: principal.Actor}
	}
	ctx = context.WithValue(ctx, principalKey{}, principal)
	return auth.WithUser(httpx.WithActor(ctx, principal.Actor), principal.User), nil
}

// authorize сверяет право операции из контракта с правами клиента. У операции с точкой или
// компанией в адресе права определяет organization: владение и назначение, а не токен (ADR-013).
// Операция без точки и компании в адресе проверяется по ролям токена: это список точек генератора.
//
// Клиент с правом generator.run - генератор событий: пока он включён на точке, он получает
// на ней и в её компании права, которые возвращает gen. Выключение отзывает их сразу.
// Клиент с правом migom.integration - сервис доставки Мигом: так же получает права на точке,
// где владелец его подключил.
// Клиент с ключом поставщика (право supplier.access) получает регистрацию поставки на своих работающих
// точках, а операции поставщика без точки в адресе - только пока его компания работает.
func authorize(org *organization.Service, gen *generator.Service, mig *migom.Service, sup *suppliers.Service) oas.Middleware {
	return func(req middleware.Request, next middleware.Next) (middleware.Response, error) {
		principal, _ := req.Context.Value(principalKey{}).(auth.Principal)
		granted := principal.Has
		pointID, onPoint := req.Params.Path("pointId")
		companyID, inCompany := req.Params.Path("companyId")
		if onPoint || inCompany {
			var permissions []string
			var err error
			// точки или компании нет - первым отвечает organization: 404
			var extra []string
			if onPoint {
				permissions, err = org.PointPermissions(req.Context, principal.User.ID, pointID.(uuid.UUID))
				if err == nil && principal.Has(generator.PermissionRun) {
					extra, err = generatorOnPoint(req.Context, org, gen, pointID.(uuid.UUID))
				}
				if err == nil && principal.Has(migom.PermissionIntegration) {
					var more []string
					more, err = migomOnPoint(req.Context, org, mig, pointID.(uuid.UUID))
					extra = append(extra, more...)
				}
				if supplier, ok := suppliers.FromActor(principal.Actor); err == nil && ok && principal.Has(suppliers.PermissionAccess) {
					var more []string
					more, err = sup.PointPermissions(req.Context, supplier, pointID.(uuid.UUID))
					extra = append(extra, more...)
				}
			} else {
				permissions, err = org.CompanyPermissions(req.Context, principal.User.ID, companyID.(uuid.UUID))
				if err == nil && principal.Has(generator.PermissionRun) {
					extra, err = generatorInCompany(req.Context, org, gen, companyID.(uuid.UUID))
				}
			}
			if err != nil {
				return middleware.Response{}, err
			}
			permissions = append(permissions, extra...)
			granted = func(permission string) bool { return slices.Contains(permissions, permission) }
		} else if supplier, ok := suppliers.FromActor(principal.Actor); ok && principal.Has(suppliers.PermissionAccess) {
			// у отключённой компании поставщику недоступно ничего
			_, err := sup.Company(req.Context, supplier)
			if errors.Is(err, organization.ErrDisabled) {
				granted = func(string) bool { return false }
			} else if err != nil {
				return middleware.Response{}, err
			}
		}
		for _, permission := range oas.GetRolesForBearerAuth(req.OperationName) {
			if !granted(permission) {
				return middleware.Response{}, &deniedError{
					operation: req.OperationName, actor: principal.Actor,
					err: &forbiddenError{actor: principal.Actor, permission: permission},
				}
			}
		}
		resp, err := next(req)
		// отказ правила предметной области (управляющий и управляющие) - тот же отказ в доступе
		if errors.Is(err, organization.ErrForbidden) {
			err = &deniedError{operation: req.OperationName, actor: principal.Actor, err: err}
		}
		return resp, err
	}
}

// generatorOnPoint возвращает права генератора на точке. На отключённой точке и на точке
// отключённой компании их нет, даже если генератор на ней остался включённым.
func generatorOnPoint(ctx context.Context, org *organization.Service, gen *generator.Service, pointID uuid.UUID) ([]string, error) {
	working, err := org.Working(ctx, pointID)
	if err != nil || !working {
		return nil, err
	}
	return gen.PointPermissions(ctx, pointID)
}

// migomOnPoint возвращает права Мигома на точке: только пока он подключён, а точка и компания работают.
func migomOnPoint(ctx context.Context, org *organization.Service, mig *migom.Service, pointID uuid.UUID) ([]string, error) {
	working, err := org.Working(ctx, pointID)
	if err != nil || !working {
		return nil, err
	}
	return mig.PointPermissions(ctx, pointID)
}

// generatorInCompany возвращает права генератора в компании: только пока она работает.
func generatorInCompany(ctx context.Context, org *organization.Service, gen *generator.Service, companyID uuid.UUID) ([]string, error) {
	company, err := org.Company(ctx, companyID)
	if err != nil || !company.Active {
		return nil, err
	}
	return gen.CompanyPermissions(ctx, companyID)
}

// allowAnonymous подставляет запросу без токена пустой токен: в режиме без авторизации
// проверка принимает любой, но сгенерированный сервер требует сам заголовок.
func allowAnonymous() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				r = r.Clone(r.Context())
				r.Header.Set("Authorization", "Bearer anonymous")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// pointStream обслуживает поток обновлений точки: ручной маршрут, который не обслуживает ogen,
// поэтому токен и право проверяются здесь. Подписаться может тот, у кого есть point.view
// на точке: владелец её компании и её сотрудники. Точка и её компания передаются потоку
// как адресат сигналов.
func pointStream(
	verifier auth.Verifier, allowAnonymous bool, org *organization.Service, logger *slog.Logger, next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		scope, actor, err := streamScope(r, verifier, allowAnonymous, org)
		if err != nil {
			problem := problemFor(err)
			if problem.Status == http.StatusUnauthorized || problem.Status == http.StatusForbidden {
				logger.WarnContext(ctx, "access denied", "operation", oas.StreamEventsOperation, "error", err,
					"actor_type", actor.Type, "actor_id", actor.ID)
			}
			if problem.Status == http.StatusUnauthorized {
				w.Header().Set("WWW-Authenticate", "Bearer")
			}
			httpx.WriteProblem(ctx, w, problem)
			return
		}
		next.ServeHTTP(w, r.WithContext(realtime.WithScope(httpx.WithActor(ctx, actor), scope)))
	})
}

func streamScope(
	r *http.Request, verifier auth.Verifier, allowAnonymous bool, org *organization.Service,
) (realtime.Scope, eventstore.Actor, error) {
	ctx := r.Context()
	token, err := bearerToken(r.Header.Get("Authorization"), allowAnonymous)
	if err != nil {
		return realtime.Scope{}, eventstore.Actor{}, err
	}
	principal, err := verifier.Verify(ctx, token)
	if err != nil {
		return realtime.Scope{}, eventstore.Actor{}, err
	}
	pointID, err := uuid.Parse(r.PathValue("pointId"))
	if err != nil {
		return realtime.Scope{}, principal.Actor, errBadPointID
	}
	permissions, err := org.PointPermissions(ctx, principal.User.ID, pointID)
	if err != nil {
		return realtime.Scope{}, principal.Actor, err
	}
	if !slices.Contains(permissions, organization.PermissionPointView) {
		return realtime.Scope{}, principal.Actor, &forbiddenError{actor: principal.Actor, permission: organization.PermissionPointView}
	}
	point, err := org.Point(ctx, pointID)
	if err != nil {
		return realtime.Scope{}, principal.Actor, err
	}
	return realtime.Scope{PointID: point.ID, CompanyID: point.CompanyID}, principal.Actor, nil
}

func bearerToken(header string, allowAnonymous bool) (string, error) {
	if header == "" && allowAnonymous {
		return "anonymous", nil
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errNoToken
	}
	return parts[1], nil
}

// challenge добавляет к ответу 401 заголовок WWW-Authenticate: сгенерированный
// ответ об ошибке заголовков не несёт.
func challenge() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(challengeWriter{w}, r)
		})
	}
}

type challengeWriter struct{ http.ResponseWriter }

func (w challengeWriter) WriteHeader(status int) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap даёт http.ResponseController доступ к исходному ResponseWriter.
func (w challengeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// isUnauthorized: токена нет или он не прошёл проверку.
func isUnauthorized(err error) bool {
	return errors.Is(err, auth.ErrInvalidToken) || errors.Is(err, errNoToken)
}
