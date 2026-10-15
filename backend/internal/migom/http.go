package migom

import (
	"context"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/organization"
)

// HTTP реализует операции подключения Мигома из api/openapi.yaml.
type HTTP struct {
	svc    *Service
	points *organization.Service
}

func NewHTTP(svc *Service, points *organization.Service) *HTTP {
	return &HTTP{svc: svc, points: points}
}

// GetMigomOnPoint реализует GET /points/{pointId}/migom.
func (h *HTTP) GetMigomOnPoint(ctx context.Context, params oas.GetMigomOnPointParams) (*oas.MigomOnPoint, error) {
	outlet, err := h.svc.Outlet(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	return migomOnPoint(outlet), nil
}

// SetMigomOnPoint реализует PUT /points/{pointId}/migom.
func (h *HTTP) SetMigomOnPoint(ctx context.Context, req *oas.MigomOnPoint, params oas.SetMigomOnPointParams) (*oas.MigomOnPoint, error) {
	// подключить можно только работающую точку; отключить - любую, иначе Мигом на точке,
	// которую потом отключили, было бы нечем отозвать
	var outlet *int64
	if v, ok := req.OutletID.Get(); ok {
		outlet = &v
	}
	find := h.points.WorkingPoint
	if outlet == nil {
		find = h.points.Point
	}
	if _, err := find(ctx, params.PointId); err != nil {
		return nil, err
	}
	if err := h.svc.Set(ctx, params.PointId, outlet); err != nil {
		return nil, err
	}
	return migomOnPoint(outlet), nil
}

// ListMigomPoints реализует GET /migom/points: пары "точка - заведение" работающих точек.
func (h *HTTP) ListMigomPoints(ctx context.Context) ([]oas.MigomPoint, error) {
	points, err := h.svc.ConnectedPoints(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]oas.MigomPoint, 0, len(points))
	for _, point := range points {
		// на отключённой точке подключение остаётся, но прав у Мигома там нет
		working, err := h.points.Working(ctx, point.PointID)
		if err != nil {
			return nil, err
		}
		if working {
			out = append(out, oas.MigomPoint{PointID: point.PointID, OutletID: point.OutletID})
		}
	}
	return out, nil
}

func migomOnPoint(outlet *int64) *oas.MigomOnPoint {
	out := &oas.MigomOnPoint{}
	if outlet != nil {
		out.OutletID.SetTo(*outlet)
	} else {
		out.OutletID.SetToNull()
	}
	return out
}
