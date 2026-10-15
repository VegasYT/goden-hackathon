// Package generator - генератор событий: настройки, которыми им управляют через API,
// и симулятор, который эмулирует работу ресторана запросами к этому же API (ADR-012).
//
// Настройки и симулятор не зависят друг от друга: первые использует cmd/api,
// второй - cmd/generator. Симулятор не имеет доступа к базе данных.
package generator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/generator/db"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
)

// MaxPerMinute - наибольшая интенсивность поставок и заказов.
const MaxPerMinute = 600

// ValidationError - некорректное поле настроек.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("generator: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение: по этому методу
// слой API распознаёт ошибку валидации, не зная о типах модулей.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

// Settings - настройки генератора.
type Settings struct {
	// Running - создаёт ли генератор поставки и заказы.
	Running             bool
	DeliveriesPerMinute int32
	OrdersPerMinute     int32
	UpdatedAt           time.Time
}

func (s Settings) validate() error {
	rate := fmt.Sprintf("must be between 0 and %d", MaxPerMinute)
	switch {
	case s.DeliveriesPerMinute < 0 || s.DeliveriesPerMinute > MaxPerMinute:
		return &ValidationError{Field: "deliveries_per_minute", Message: rate}
	case s.OrdersPerMinute < 0 || s.OrdersPerMinute > MaxPerMinute:
		return &ValidationError{Field: "orders_per_minute", Message: rate}
	}
	return nil
}

// PermissionRun - право учётной записи генератора: получать список точек, на которых он включён.
// Выдаётся в Keycloak только сервисной учётной записи генератора.
const PermissionRun = "generator.run"

// Права, которые клиент с PermissionRun получает, пока генератор включён: на точке - создавать
// поставки и заказы и читать, какие блюда на ней продаются, в её компании - читать каталог
// и наполнять его, если он пуст.
// После выключения генератора на точке эти права пропадают сразу.
var (
	pointPermissions   = []string{"catalog.view", "deliveries.view", "deliveries.register", "deliveries.approve", "orders.accept"}
	companyPermissions = []string{"catalog.view", "products.manage", "dishes.manage"}
)

// defaults - настройки точки, для которой их ещё не задавали: генератор остановлен.
var defaults = Settings{DeliveriesPerMinute: 6, OrdersPerMinute: 30}

// RunningPoint - точка, на которой генератор включён.
type RunningPoint struct {
	PointID, CompanyID  uuid.UUID
	DeliveriesPerMinute int32
	OrdersPerMinute     int32
}

// Service хранит настройки генератора по точкам.
type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, clock: clk}
}

// Settings возвращает настройки генератора на точке.
func (s *Service) Settings(ctx context.Context, pointID uuid.UUID) (Settings, error) {
	row, err := db.New(s.pool).GetPointSettings(ctx, pointID)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaults, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("generator: read settings: %w", err)
	}
	return Settings{
		Running: row.Running, DeliveriesPerMinute: row.DeliveriesPerMinute,
		OrdersPerMinute: row.OrdersPerMinute, UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

// Update заменяет настройки генератора на точке. Генератор подхватит их при следующем опросе.
func (s *Service) Update(ctx context.Context, point organization.Point, in Settings) (Settings, error) {
	if err := in.validate(); err != nil {
		return Settings{}, err
	}
	row, err := db.New(s.pool).UpsertPointSettings(ctx, db.UpsertPointSettingsParams{
		PointID: point.ID, CompanyID: point.CompanyID, Running: in.Running,
		DeliveriesPerMinute: in.DeliveriesPerMinute, OrdersPerMinute: in.OrdersPerMinute, UpdatedAt: s.clock.Now(),
	})
	if err != nil {
		return Settings{}, fmt.Errorf("generator: save settings: %w", err)
	}
	return Settings{
		Running: row.Running, DeliveriesPerMinute: row.DeliveriesPerMinute,
		OrdersPerMinute: row.OrdersPerMinute, UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

// RunningPoints возвращает точки, на которых генератор включён.
func (s *Service) RunningPoints(ctx context.Context) ([]RunningPoint, error) {
	rows, err := db.New(s.pool).ListRunningPoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("generator: list running points: %w", err)
	}
	points := make([]RunningPoint, len(rows))
	for i, row := range rows {
		points[i] = RunningPoint(row)
	}
	return points, nil
}

// PointPermissions возвращает права учётной записи генератора на точке: непустые, только пока
// генератор на ней включён.
func (s *Service) PointPermissions(ctx context.Context, pointID uuid.UUID) ([]string, error) {
	running, err := db.New(s.pool).RunsOnPoint(ctx, pointID)
	if err != nil {
		return nil, fmt.Errorf("generator: read settings: %w", err)
	}
	if !running {
		return nil, nil
	}
	return pointPermissions, nil
}

// CompanyPermissions возвращает права учётной записи генератора в компании: непустые, только пока
// генератор включён хотя бы на одной её точке.
func (s *Service) CompanyPermissions(ctx context.Context, companyID uuid.UUID) ([]string, error) {
	running, err := db.New(s.pool).RunsInCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("generator: read settings: %w", err)
	}
	if !running {
		return nil, nil
	}
	return companyPermissions, nil
}
