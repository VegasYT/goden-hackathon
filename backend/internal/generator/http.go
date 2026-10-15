package generator

import (
	"context"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/organization"
)

// HTTP реализует операции настроек генератора из api/openapi.yaml.
type HTTP struct {
	svc    *Service
	points *organization.Service
}

func NewHTTP(svc *Service, points *organization.Service) *HTTP {
	return &HTTP{svc: svc, points: points}
}

// GetGeneratorSettings реализует GET /points/{pointId}/generator.
func (h *HTTP) GetGeneratorSettings(ctx context.Context, params oas.GetGeneratorSettingsParams) (*oas.GeneratorSettings, error) {
	settings, err := h.svc.Settings(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	return settingsResponse(settings), nil
}

// UpdateGeneratorSettings реализует PUT /points/{pointId}/generator.
func (h *HTTP) UpdateGeneratorSettings(
	ctx context.Context, req *oas.GeneratorSettings, params oas.UpdateGeneratorSettingsParams,
) (*oas.GeneratorSettings, error) {
	// На отключённой точке генератор не включается: записи на ней всё равно отклоняются.
	// Выключить его можно на любой точке: иначе генератор, оставшийся включённым на точке,
	// которую потом отключили, было бы нечем остановить.
	find := h.points.WorkingPoint
	if !req.Running {
		find = h.points.Point
	}
	point, err := find(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	settings, err := h.svc.Update(ctx, point, Settings{
		Running: req.Running, DeliveriesPerMinute: req.DeliveriesPerMinute, OrdersPerMinute: req.OrdersPerMinute,
	})
	if err != nil {
		return nil, err
	}
	return settingsResponse(settings), nil
}

// ListGeneratorPoints реализует GET /generator/points: точки, на которых генератор включён.
func (h *HTTP) ListGeneratorPoints(ctx context.Context) ([]oas.GeneratorPoint, error) {
	points, err := h.svc.RunningPoints(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]oas.GeneratorPoint, 0, len(points))
	for _, point := range points {
		// Отключённую точку и точку отключённой компании генератор не получает: настройки
		// остаются включёнными, а записи на такой точке отклоняются.
		working, err := h.points.Working(ctx, point.PointID)
		if err != nil {
			return nil, err
		}
		if !working {
			continue
		}
		out = append(out, oas.GeneratorPoint{
			PointID: point.PointID, CompanyID: point.CompanyID,
			DeliveriesPerMinute: point.DeliveriesPerMinute, OrdersPerMinute: point.OrdersPerMinute,
		})
	}
	return out, nil
}

func settingsResponse(s Settings) *oas.GeneratorSettings {
	out := &oas.GeneratorSettings{
		Running: s.Running, DeliveriesPerMinute: s.DeliveriesPerMinute, OrdersPerMinute: s.OrdersPerMinute,
	}
	// у точки без сохранённых настроек времени изменения нет
	if !s.UpdatedAt.IsZero() {
		out.UpdatedAt = oas.NewOptDateTime(s.UpdatedAt)
	}
	return out
}
