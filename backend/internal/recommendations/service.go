package recommendations

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/stoplist"
)

// everything - страница, вмещающая весь список: расчёты идут по всем блюдам и продуктам сразу.
var everything = catalog.Page{Limit: math.MaxInt32}

// ValidationError - некорректное сочетание параметров запроса.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("recommendations: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение: по этому методу
// слой API распознаёт ошибку валидации, не зная о типах модулей.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

// Service считает рекомендации по данным меню и склада и хранит ручные горячие предложения точек.
type Service struct {
	runner *command.Runner
	pool   *pgxpool.Pool
	menu   *stoplist.Service
	stock  *inventory.Service
	now    func() time.Time
}

// NewService создаёт сервис. Срочность считается по часам команд: от того же момента,
// по которому определяется просрочка.
func NewService(runner *command.Runner, pool *pgxpool.Pool, menu *stoplist.Service, stock *inventory.Service) *Service {
	return &Service{runner: runner, pool: pool, menu: menu, stock: stock, now: runner.Now}
}

// DishOffers возвращает страницу горячих предложений точки и их общее число: сначала добавленные
// вручную, затем расчёт от самого приоритетного.
//
// упрощение: на каждый запрос читаются всё меню и все партии под риском, пагинация - в памяти.
// Предел тот же, что у стоп-листа: при тысячах блюд нужен кеш с инвалидацией по событиям.
func (s *Service) DishOffers(ctx context.Context, point organization.Point, horizon time.Duration, page catalog.Page) ([]DishOffer, int64, error) {
	menu, _, err := s.menu.Menu(ctx, point, false, everything)
	if err != nil {
		return nil, 0, err
	}
	expiring, _, err := s.stock.ExpiringBatches(ctx, point.ID, horizon, everything)
	if err != nil {
		return nil, 0, err
	}
	settings, err := s.HotOfferSettings(ctx, point.ID)
	if err != nil {
		return nil, 0, err
	}
	offers := withSettings(hotOffers(menu, expiring, s.now(), horizon), menu, settings)
	from, to := bounds(len(offers), page)
	return offers[from:to], int64(len(offers)), nil
}

// SupplyAdvice возвращает страницу рекомендаций поставок точке от самых срочных и их общее число.
func (s *Service) SupplyAdvice(
	ctx context.Context, point organization.Point, window, cover, target time.Duration, page catalog.Page,
) ([]SupplyAdvice, int64, error) {
	if target < cover {
		return nil, 0, &ValidationError{Field: "target_hours", Message: "must not be less than cover_hours"}
	}
	menu, _, err := s.menu.Menu(ctx, point, false, everything)
	if err != nil {
		return nil, 0, err
	}
	dishes := make([]catalog.Dish, len(menu))
	for i, a := range menu {
		dishes[i] = a.Dish
	}
	stock, _, err := s.stock.Stock(ctx, point, everything)
	if err != nil {
		return nil, 0, err
	}
	now := s.now()
	consumed, first, err := s.stock.Consumption(ctx, point.ID, now.Add(-window))
	if err != nil {
		return nil, 0, err
	}
	advice := supplyAdvice(stock, consumed, dishes, observedPeriod(now, first, window), cover, target)
	from, to := bounds(len(advice), page)
	return advice[from:to], int64(len(advice)), nil
}

// bounds переводит страницу в границы среза длины n.
func bounds(n int, page catalog.Page) (from, to int) {
	from = min(int(page.Offset), n)
	return from, min(from+int(page.Limit), n)
}
