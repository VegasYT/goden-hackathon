package analytics

import (
	"context"
	"time"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/organization"
	platformredis "goden/backend/internal/platform/redis"
)

// HTTP реализует операции аналитики из api/openapi.yaml.
type HTTP struct {
	service *Service
	points  *organization.Service
	cache   *platformredis.Cache
}

func NewHTTP(service *Service, points *organization.Service, caches ...*platformredis.Cache) *HTTP {
	var cache *platformredis.Cache
	if len(caches) > 0 {
		cache = caches[0]
	}
	return &HTTP{service: service, points: points, cache: cache}
}

func (h *HTTP) GetWriteOffStatistics(
	ctx context.Context, params oas.GetWriteOffStatisticsParams,
) (*oas.WriteOffStatistics, error) {
	return platformredis.LoadProjectionJSON(ctx, h.cache, "analytics", "write-offs", params,
		func(ctx context.Context) (*oas.WriteOffStatistics, error) { return h.writeOffStatistics(ctx, params) })
}

func (h *HTTP) writeOffStatistics(
	ctx context.Context, params oas.GetWriteOffStatisticsParams,
) (*oas.WriteOffStatistics, error) {
	from, to := h.period(params.From, params.To)
	statistics, err := h.service.WriteOffs(ctx, params.PointId, from, to)
	if err != nil {
		return nil, err
	}
	response := &oas.WriteOffStatistics{Items: make([]oas.WriteOffStatistic, len(statistics))}
	for i, item := range statistics {
		response.Items[i] = oas.WriteOffStatistic{
			Product: *catalog.ProductResponse(item.Product), Reason: oas.WriteOffReason(item.Reason),
			Quantity: item.Quantity, Operations: item.Operations,
		}
	}
	return response, nil
}

func (h *HTTP) GetWriteOffDays(ctx context.Context, params oas.GetWriteOffDaysParams) (*oas.WriteOffDays, error) {
	return platformredis.LoadProjectionJSON(ctx, h.cache, "analytics", "write-off-days", params,
		func(ctx context.Context) (*oas.WriteOffDays, error) { return h.writeOffDays(ctx, params) })
}

func (h *HTTP) writeOffDays(ctx context.Context, params oas.GetWriteOffDaysParams) (*oas.WriteOffDays, error) {
	from, to := h.period(params.From, params.To)
	days, err := h.service.WriteOffDays(ctx, params.PointId, from, to, params.TimeZone.Or("UTC"))
	if err != nil {
		return nil, err
	}
	response := &oas.WriteOffDays{Items: make([]oas.WriteOffDay, len(days))}
	for i, item := range days {
		response.Items[i] = oas.WriteOffDay{
			Day: item.Day, Product: *catalog.ProductResponse(item.Product), Reason: oas.WriteOffReason(item.Reason),
			Quantity: item.Quantity, Operations: item.Operations,
		}
	}
	return response, nil
}

func (h *HTTP) GetShortageStatistics(
	ctx context.Context, params oas.GetShortageStatisticsParams,
) (*oas.ShortageStatistics, error) {
	return platformredis.LoadProjectionJSON(ctx, h.cache, "analytics", "shortages", params,
		func(ctx context.Context) (*oas.ShortageStatistics, error) { return h.shortageStatistics(ctx, params) })
}

func (h *HTTP) shortageStatistics(
	ctx context.Context, params oas.GetShortageStatisticsParams,
) (*oas.ShortageStatistics, error) {
	from, to := h.period(params.From, params.To)
	statistics, err := h.service.Shortages(ctx, params.PointId, from, to)
	if err != nil {
		return nil, err
	}
	response := &oas.ShortageStatistics{Items: make([]oas.ShortageStatistic, len(statistics))}
	for i, item := range statistics {
		response.Items[i] = oas.ShortageStatistic{
			Product: *catalog.ProductResponse(item.Product), RejectedOrders: item.RejectedOrders,
			StopEntries: item.StopEntries, StopSeconds: item.StopSeconds,
		}
	}
	return response, nil
}

func (h *HTTP) GetStopListForecast(
	ctx context.Context, params oas.GetStopListForecastParams,
) (*oas.StopListForecasts, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	forecasts, err := h.service.Forecast(ctx, point, time.Duration(params.WindowHours.Value)*time.Hour,
		time.Duration(params.HorizonHours.Value)*time.Hour)
	if err != nil {
		return nil, err
	}
	response := &oas.StopListForecasts{Items: make([]oas.StopListForecast, len(forecasts))}
	for i, item := range forecasts {
		response.Items[i] = oas.StopListForecast{
			Dish:            oas.DishRef{ID: item.Dish.ID, Name: item.Dish.Name},
			LimitingProduct: *catalog.ProductResponse(item.Product), ExpectedAt: item.ExpectedAt,
			AvailableQuantity: item.Available, RequiredQuantity: item.Required,
			ConsumptionPerHour: item.ConsumptionPerHour, Cause: oas.StopListForecastCause(item.Cause),
		}
	}
	return response, nil
}

// defaultPeriod - период статистики, когда клиент не назвал его границы.
const defaultPeriod = 7 * 24 * time.Hour

// period подставляет границы по умолчанию: до текущего момента, семь суток.
func (h *HTTP) period(from, to oas.OptDateTime) (time.Time, time.Time) {
	end := to.Or(h.service.now())
	return from.Or(end.Add(-defaultPeriod)), end
}
