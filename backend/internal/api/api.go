// Package api собирает HTTP API из обработчиков доменных модулей.
//
// Контракт - api/openapi.yaml, код сервера и клиента в подпакете oas сгенерирован ogen.
// Модули реализуют свои операции в файлах http.go; здесь они объединяются в один
// обработчик и получают общие правила: формат ошибок, correlation ID, лог запросов.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ogen-go/ogen/ogenerrors"

	"goden/backend/internal/analytics"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/generator"
	"goden/backend/internal/inventory"
	"goden/backend/internal/migom"
	"goden/backend/internal/orders"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/platform/observability"
	platformredis "goden/backend/internal/platform/redis"
	"goden/backend/internal/realtime"
	"goden/backend/internal/recommendations"
	"goden/backend/internal/stoplist"
	"goden/backend/internal/suppliers"
)

// Prefix - базовый путь API из раздела servers контракта.
const Prefix = "/api/v1"

// maxBodyBytes ограничивает тело запроса: API принимает небольшие JSON-документы.
const maxBodyBytes = 1 << 20

// Обработчики модулей называются одинаково, а имя встроенного поля берётся
// из имени типа, поэтому встраиваются они под своими псевдонимами.
type (
	catalogHTTP   = catalog.HTTP
	inventoryHTTP = inventory.HTTP
	ordersHTTP    = orders.HTTP
	generatorHTTP = generator.HTTP
	migomHTTP     = migom.HTTP
	supplierHTTP  = suppliers.HTTP
	stoplistHTTP  = stoplist.HTTP
	adviceHTTP    = recommendations.HTTP
	analyticsHTTP = analytics.HTTP
	orgHTTP       = organization.HTTP
)

// Handler реализует все операции контракта встраиванием обработчиков модулей.
type Handler struct {
	*catalogHTTP
	*inventoryHTTP
	*ordersHTTP
	*generatorHTTP
	*migomHTTP
	*supplierHTTP
	*stoplistHTTP
	*adviceHTTP
	*analyticsHTTP
	*orgHTTP
	// Заглушки лежат на уровень глубже обработчиков модулей, поэтому метод модуля
	// побеждает заглушку с тем же именем. Операция, которая есть в контракте,
	// но ещё не реализована, отвечает 501 и не ломает сборку.
	unimplemented

	logger *slog.Logger
}

type unimplemented struct{ oas.UnimplementedHandler }

// NewError превращает ошибку обработчика в ответ по RFC 9457.
func (h *Handler) NewError(ctx context.Context, err error) *oas.ErrorStatusCode {
	problem := problemFor(err)
	switch {
	case problem.Status >= http.StatusInternalServerError:
		// клиенту текст внутренней ошибки не уходит, поэтому он обязан попасть в лог
		h.logger.ErrorContext(ctx, "request failed", "error", err)
	case problem.Status == http.StatusUnauthorized || problem.Status == http.StatusForbidden:
		h.logDenied(ctx, err)
	}
	return &oas.ErrorStatusCode{StatusCode: problem.Status, Response: problemResponse(ctx, problem)}
}

// logDenied пишет отказ в доступе: операцию, причину и автора, если токен его назвал.
func (h *Handler) logDenied(ctx context.Context, err error) {
	attrs := []any{"error", err}
	var (
		denied      *deniedError
		securityErr *ogenerrors.SecurityError
		forbidden   *forbiddenError
	)
	if errors.As(err, &denied) {
		attrs = append(attrs, "operation", denied.operation, "actor_type", denied.actor.Type, "actor_id", denied.actor.ID)
	} else {
		if errors.As(err, &securityErr) {
			attrs = append(attrs, "operation", securityErr.OperationName())
		}
		if errors.As(err, &forbidden) {
			attrs = append(attrs, "actor_type", forbidden.actor.Type, "actor_id", forbidden.actor.ID)
		}
	}
	h.logger.WarnContext(ctx, "access denied", attrs...)
}

// Options - настройки доступа к API.
type Options struct {
	// Verifier проверяет токен запроса.
	Verifier auth.Verifier
	// AllowAnonymous пропускает запросы без токена к Verifier: режим без авторизации.
	AllowAnonymous bool
	// CORSOrigins - адреса frontend, которым разрешены кросс-доменные запросы.
	CORSOrigins []string
	// Realtime раздаёт SSE-сигналы и связывается с Kafka-reader процесса API.
	Realtime *realtime.Hub
	// ReadCache ускоряет только согласованные асинхронные read-модели.
	ReadCache *platformredis.Cache
}

// New собирает сервисы и возвращает обработчик всего процесса API:
// операции контракта под Prefix, /healthz и /readyz, общие middleware.
func New(pool *pgxpool.Pool, clk clock.Clock, logger *slog.Logger, opts Options) (http.Handler, error) {
	if opts.Realtime == nil {
		opts.Realtime = realtime.NewHub()
	}
	runner := command.NewRunner(pool, clk, logger)
	if opts.ReadCache != nil {
		opts.ReadCache.Register(runner.Registry())
	}
	orgService := organization.NewService(runner, pool)
	catalogService := catalog.NewService(runner, pool)
	inventoryService := inventory.NewService(runner, pool, catalogService)
	// единицу продукта после первой поставки менять нельзя: это знает склад
	catalogService.SetProductUsage(inventoryService)
	ordersService := orders.NewService(runner, pool, catalogService, inventoryService)

	stoplistService := stoplist.NewService(catalogService, inventoryService)
	generatorService := generator.NewService(pool, clk)
	migomService := migom.NewService(pool, clk)
	supplierService := suppliers.NewService(pool, clk, orgService, logger)
	// названия поставщиков в авторах ответов; suppliers сам зависит от organization
	orgService.SetServiceNames(supplierService)
	// ключ поставщика проверяется здесь, остальные токены - переданной проверкой (ADR-014)
	verifier := supplierService.Verifier(opts.Verifier)

	handler := &Handler{
		catalogHTTP:   catalog.NewHTTP(catalogService, orgService),
		inventoryHTTP: inventory.NewHTTP(inventoryService, orgService, opts.ReadCache),
		ordersHTTP:    orders.NewHTTP(ordersService, orgService, opts.ReadCache),
		generatorHTTP: generator.NewHTTP(generatorService, orgService),
		migomHTTP:     migom.NewHTTP(migomService, orgService),
		supplierHTTP:  suppliers.NewHTTP(supplierService, catalogService),
		stoplistHTTP:  stoplist.NewHTTP(stoplistService, orgService),
		adviceHTTP:    recommendations.NewHTTP(recommendations.NewService(runner, pool, stoplistService, inventoryService), orgService, catalogService),
		analyticsHTTP: analytics.NewHTTP(analytics.NewService(pool, catalogService, inventoryService, runner.Now), orgService, opts.ReadCache),
		orgHTTP:       organization.NewHTTP(orgService),
		logger:        logger,
	}
	server, err := oas.NewServer(handler, security{verifier: verifier},
		oas.WithPathPrefix(Prefix),
		oas.WithMiddleware(authorize(orgService, generatorService, migomService, supplierService)),
		oas.WithErrorHandler(requestErrorHandler(logger)),
		oas.WithNotFound(notFoundHandler),
		oas.WithMethodNotAllowed(methodNotAllowedHandler),
	)
	if err != nil {
		return nil, err
	}

	registry := runner.Registry()
	observability.RegisterPool(registry, pool)

	// метрики стоят снаружи: в них попадают и запросы, отклонённые до обработчика операции
	// Recover стоит внутри метрик и журнала запросов: они пишут после обработчика, и паника,
	// перехваченная снаружи, проходила мимо них - запрос с ответом 500 не попадал ни в счётчик, ни в журнал.
	middlewares := []httpx.Middleware{
		requestMetrics(registry, server), httpx.Recover(logger), httpx.TextBody(maxBodyBytes), challenge(),
	}
	if opts.AllowAnonymous {
		middlewares = append(middlewares, allowAnonymous())
	}
	mux := httpx.Health(pool.Ping)
	mux.Handle("GET /metrics", observability.MetricsHandler(registry))
	// шаблон конкретнее общего Prefix+"/", поэтому поток обслуживает этот маршрут, а не ogen
	mux.Handle("GET "+Prefix+"/points/{pointId}/events", pointStream(
		verifier, opts.AllowAnonymous, orgService, logger, realtime.NewHandler(opts.Realtime),
	))
	mux.Handle(Prefix+"/", httpx.Chain(server, middlewares...))

	return httpx.Chain(mux,
		httpx.Correlation(), httpx.AccessLog(logger), httpx.Recover(logger), httpx.CORS(opts.CORSOrigins),
	), nil
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	httpx.WriteProblem(r.Context(), w, httpx.Problem{
		Type: "not-found", Title: "Путь не найден", Status: http.StatusNotFound,
		Detail: "путь не существует: " + r.URL.Path,
	})
}

func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request, allowed string) {
	w.Header().Set("Allow", allowed)
	httpx.WriteProblem(r.Context(), w, httpx.Problem{
		Type: "method-not-allowed", Title: "Метод не разрешён", Status: http.StatusMethodNotAllowed,
		Detail: "HTTP-метод не поддерживается для этого пути",
	})
}

// requestErrorHandler обрабатывает ошибки, возникшие до вызова обработчика операции:
// запрос не разобран или не прошёл проверку по контракту.
func requestErrorHandler(logger *slog.Logger) oas.ErrorHandler {
	return func(ctx context.Context, w http.ResponseWriter, _ *http.Request, err error) {
		status := ogenerrors.ErrorCode(err)
		if status >= http.StatusInternalServerError {
			logger.ErrorContext(ctx, "request failed", "error", err)
			httpx.WriteProblem(ctx, w, internalProblem)
			return
		}
		httpx.WriteProblem(ctx, w, httpx.Problem{
			Type: "bad-request", Title: "Запрос не соответствует контракту API", Status: status,
			Detail: "проверьте параметры и тело запроса",
		})
	}
}
