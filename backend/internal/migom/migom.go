// Package migom - подключение сервиса доставки Мигом к точке: владелец указывает на своей точке
// заведение Мигома, и учётная запись Мигома получает на ней права читать меню и оформлять и отменять
// заказы. Учётная запись одна на все заведения, поэтому Мигом сверяет пару "точка - заведение"
// с точкой, которую указал владелец заведения у себя.
package migom

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/migom/db"
	"goden/backend/internal/platform/clock"
)

// PermissionIntegration - роль токена учётной записи Мигома (клиент migom-integration).
const PermissionIntegration = "migom.integration"

// pointPermissions - права учётной записи Мигома на точке, где он подключён: меню, горячие
// предложения для покупателей, оформление, чтение и отмена заказа. Больше ничего Мигому не нужно.
var pointPermissions = []string{"point.view", "kitchen.view", "orders.accept", "orders.cancel", "orders.view", "recommendations.view"}

type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, clock: clk}
}

// Point - точка, к которой подключено заведение Мигома.
type Point struct {
	PointID  uuid.UUID
	OutletID int64
}

// Outlet возвращает номер заведения Мигома, подключённого к точке, или nil, если Мигом не подключён.
func (s *Service) Outlet(ctx context.Context, pointID uuid.UUID) (*int64, error) {
	outlet, err := db.New(s.pool).OutletOnPoint(ctx, pointID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("migom: read point setting: %w", err)
	}
	return outlet, nil
}

// Set подключает к точке заведение Мигома; nil отключает Мигом.
func (s *Service) Set(ctx context.Context, pointID uuid.UUID, outletID *int64) error {
	err := db.New(s.pool).SetOutletOnPoint(ctx, db.SetOutletOnPointParams{PointID: pointID, OutletID: outletID, UpdatedAt: s.clock.Now().UTC()})
	if err != nil {
		return fmt.Errorf("migom: save point setting: %w", err)
	}
	return nil
}

// ConnectedPoints возвращает все точки с подключённым заведением; работают ли они, проверяет вызывающий.
func (s *Service) ConnectedPoints(ctx context.Context) ([]Point, error) {
	rows, err := db.New(s.pool).ListConnectedPoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("migom: list points: %w", err)
	}
	points := make([]Point, len(rows))
	for i, row := range rows {
		points[i] = Point{PointID: row.PointID, OutletID: row.OutletID}
	}
	return points, nil
}

// PointPermissions возвращает права учётной записи Мигома на точке: непустые, только пока он подключён.
// Работает ли точка, проверяет вызывающий.
func (s *Service) PointPermissions(ctx context.Context, pointID uuid.UUID) ([]string, error) {
	outlet, err := s.Outlet(ctx, pointID)
	if err != nil || outlet == nil {
		return nil, err
	}
	return pointPermissions, nil
}
