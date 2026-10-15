package analytics

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"
	// база поясов в бинарнике: проверка пояса не зависит от образа контейнера
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/analytics/db"
	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
)

// InvalidPeriodError означает, что полуоткрытый период задан в обратном порядке.
type InvalidPeriodError struct {
	From time.Time
	To   time.Time
}

func (e *InvalidPeriodError) Error() string { return "analytics: from must be earlier than to" }

// InvalidTimeZoneError означает, что клиент передал неизвестный часовой пояс.
type InvalidTimeZoneError struct{ Name string }

func (e *InvalidTimeZoneError) Error() string { return "analytics: unknown time zone " + e.Name }

// WriteOffDay - списания продукта по одной причине за день в часовом поясе запроса.
type WriteOffDay struct {
	Day time.Time
	WriteOffStatistic
}

type WriteOffStatistic struct {
	Product    catalog.Product
	Reason     inventory.WriteOffReason
	Quantity   int64
	Operations int64
}

type ShortageStatistic struct {
	Product        catalog.Product
	RejectedOrders int64
	StopEntries    int64
	StopSeconds    int64
}

type Service struct {
	pool     *pgxpool.Pool
	products *catalog.Service
	stock    *inventory.Service
	now      func() time.Time
}

func NewService(pool *pgxpool.Pool, products *catalog.Service, stock *inventory.Service, now func() time.Time) *Service {
	return &Service{pool: pool, products: products, stock: stock, now: now}
}

// WriteOffs возвращает статистику списаний точки за период.
func (s *Service) WriteOffs(ctx context.Context, pointID uuid.UUID, from, to time.Time) ([]WriteOffStatistic, error) {
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).WriteOffStatistics(ctx, db.WriteOffStatisticsParams{PointID: pointID, FromTime: from.UTC(), ToTime: to.UTC()})
	if err != nil {
		return nil, fmt.Errorf("analytics: write-off statistics: %w", err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ProductID
	}
	products, err := s.products.ProductsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]WriteOffStatistic, len(rows))
	for i, row := range rows {
		product, ok := products[row.ProductID]
		if !ok {
			return nil, fmt.Errorf("analytics: product %s is absent from catalog", row.ProductID)
		}
		result[i] = WriteOffStatistic{
			Product: product, Reason: inventory.WriteOffReason(row.Reason),
			Quantity: row.Quantity, Operations: row.Operations,
		}
	}
	// Количества в разных единицах несравнимы (5000 г и 40 шт), поэтому порядок - внутри единицы.
	slices.SortFunc(result, func(a, b WriteOffStatistic) int {
		return cmp.Or(cmp.Compare(a.Product.Unit, b.Product.Unit), cmp.Compare(b.Quantity, a.Quantity),
			cmp.Compare(a.Product.Name, b.Product.Name), cmp.Compare(a.Reason, b.Reason))
	})
	return result, nil
}

// WriteOffDays возвращает списания точки за период по дням часового пояса timeZone.
func (s *Service) WriteOffDays(
	ctx context.Context, pointID uuid.UUID, from, to time.Time, timeZone string,
) ([]WriteOffDay, error) {
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	// пустое имя LoadLocation понимает как UTC, а PostgreSQL - нет
	if _, err := time.LoadLocation(timeZone); err != nil || timeZone == "" {
		return nil, &InvalidTimeZoneError{Name: timeZone}
	}
	rows, err := db.New(s.pool).WriteOffDays(ctx, db.WriteOffDaysParams{
		TimeZone: timeZone, PointID: pointID, FromTime: from.UTC(), ToTime: to.UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("analytics: write-off days: %w", err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ProductID
	}
	products, err := s.products.ProductsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]WriteOffDay, len(rows))
	for i, row := range rows {
		product, ok := products[row.ProductID]
		if !ok {
			return nil, fmt.Errorf("analytics: product %s is absent from catalog", row.ProductID)
		}
		day, err := time.Parse(time.DateOnly, row.Day)
		if err != nil {
			return nil, fmt.Errorf("analytics: write-off day %q: %w", row.Day, err)
		}
		result[i] = WriteOffDay{Day: day, WriteOffStatistic: WriteOffStatistic{
			Product: product, Reason: inventory.WriteOffReason(row.Reason),
			Quantity: row.Quantity, Operations: row.Operations,
		}}
	}
	return result, nil
}

// Shortages возвращает статистику дефицита точки за период.
func (s *Service) Shortages(ctx context.Context, pointID uuid.UUID, from, to time.Time) ([]ShortageStatistic, error) {
	if err := validPeriod(from, to); err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ShortageStatistics(ctx, db.ShortageStatisticsParams{
		PointID: pointID, FromTime: from.UTC(), ToTime: to.UTC(), NowTime: s.now().UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("analytics: shortage statistics: %w", err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ProductID
	}
	products, err := s.products.ProductsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]ShortageStatistic, len(rows))
	for i, row := range rows {
		product, ok := products[row.ProductID]
		if !ok {
			return nil, fmt.Errorf("analytics: product %s is absent from catalog", row.ProductID)
		}
		result[i] = ShortageStatistic{
			Product: product, RejectedOrders: row.RejectedOrders,
			StopEntries: row.StopEntries, StopSeconds: row.StopSeconds,
		}
	}
	slices.SortFunc(result, func(a, b ShortageStatistic) int {
		// главный ключ - время в стоп-листе: число отказов накручивают заведомо завышенные заказы
		return cmp.Or(cmp.Compare(b.StopSeconds, a.StopSeconds), cmp.Compare(b.StopEntries, a.StopEntries),
			cmp.Compare(b.RejectedOrders, a.RejectedOrders), cmp.Compare(a.Product.Name, b.Product.Name))
	})
	return result, nil
}

func validPeriod(from, to time.Time) error {
	if !from.Before(to) {
		return &InvalidPeriodError{From: from, To: to}
	}
	return nil
}
