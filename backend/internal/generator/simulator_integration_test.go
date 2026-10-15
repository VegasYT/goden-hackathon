//go:build integration

package generator_test

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/generator"
	"goden/backend/internal/platform/eventstore"
)

var ctx = context.Background()

const (
	tick    = 20 * time.Millisecond
	settle  = 500 * time.Millisecond // с запасом больше периода опроса и одного действия
	waitFor = 10 * time.Second
)

var (
	deliveryEvents = []string{"DeliveryRegistered"}
	orderEvents    = []string{"OrderAccepted", "OrderRejected"}
)

// site - точка пользователя dev: на ней симулятор создаёт поставки и заказы в тестах.
func site(s apitest.Server) generator.Site {
	return generator.Site{PointID: s.PointID, CompanyID: s.CompanyID}
}

// newSimulator создаёт симулятор с фиксированным источником случайности: прогон воспроизводим.
func newSimulator(s apitest.Server, client generator.API) *generator.Simulator {
	return generator.NewSimulator(client, s.Clock, rand.New(rand.NewPCG(1, 2)), slog.New(slog.DiscardHandler))
}

func countEvents(t *testing.T, s apitest.Server, types []string) int {
	t.Helper()
	var n int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE event_type = ANY($1)`, types).Scan(&n))
	return n
}

func setSettings(t *testing.T, s apitest.Server, running bool, deliveries, orders int32) {
	t.Helper()
	_, err := s.Client.UpdateGeneratorSettings(ctx, &oas.GeneratorSettings{
		Running: running, DeliveriesPerMinute: deliveries, OrdersPerMinute: orders,
	}, oas.UpdateGeneratorSettingsParams{PointId: s.PointID})
	require.NoError(t, err)
}

// run запускает цикл симулятора и останавливает его в конце теста.
func run(t *testing.T, sim *generator.Simulator) {
	t.Helper()
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- sim.Run(runCtx, tick, tick) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(waitFor):
			t.Error("Run did not stop after the context was cancelled")
		}
	})
}

func TestSeed(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)

	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Positive(t, products.Total)
	for _, product := range products.Items {
		require.True(t, product.ShelfLifeDays.Set, product.Name)
	}
	dishes, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Positive(t, dishes.Total)
	for _, dish := range dishes.Items {
		require.NotEmpty(t, dish.Ingredients, dish.Name)
	}

	// повторный вызов и новый процесс ничего не добавляют
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	require.NoError(t, newSimulator(s, s.Client).Seed(ctx, s.CompanyID))
	again, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, products.Total, again.Total)
	dishesAgain, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, dishes.Total, dishesAgain.Total)
}

// Генератор работает только там, где его включили: на своей точке с её каталогом,
// не трогая соседнюю точку и чужую компанию.
func TestRunWorksOnlyOnEnabledPoints(t *testing.T) {
	s := apitest.New(t)
	quiet, err := s.Client.CreatePoint(ctx, &oas.NewPoint{Name: "Тихая"}, oas.CreatePointParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	// чужая компания с точкой: генератор на ней не включён
	stranger := s.Login(t, "stranger")
	foreign, err := stranger.CreateCompany(ctx, &oas.NewCompany{Name: "Чужая"}, oas.CreateCompanyParams{})
	require.NoError(t, err)
	foreignPoint, err := stranger.CreatePoint(ctx, &oas.NewPoint{Name: "Чужая точка"}, oas.CreatePointParams{CompanyId: foreign.ID})
	require.NoError(t, err)

	sim := newSimulator(s, s.Client)
	sim.SeedCatalog = true
	run(t, sim)
	setSettings(t, s, true, 600, 600)

	onPoint := func(pointID uuid.UUID, types []string) (n int) {
		require.NoError(t, s.Pool.QueryRow(ctx,
			`SELECT count(*) FROM events WHERE event_type = ANY($1) AND payload->>'point_id' = $2`, types, pointID.String()).Scan(&n))
		return n
	}
	require.Eventually(t, func() bool {
		return onPoint(s.PointID, deliveryEvents) >= 3 && onPoint(s.PointID, orderEvents) >= 3
	}, waitFor, tick)
	for _, point := range []uuid.UUID{quiet.ID, foreignPoint.ID} {
		require.Zero(t, onPoint(point, deliveryEvents))
		require.Zero(t, onPoint(point, orderEvents))
	}

	// каталог наполнен в компании включённой точки и только в ней
	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Positive(t, products.Total)
	foreignProducts, err := stranger.ListProducts(ctx, oas.ListProductsParams{CompanyId: foreign.ID})
	require.NoError(t, err)
	require.Zero(t, foreignProducts.Total)

	// включили на второй точке: она получает свои поставки, каталог второй раз не наполняется
	_, err = s.Client.UpdateGeneratorSettings(ctx, &oas.GeneratorSettings{Running: true, DeliveriesPerMinute: 600, OrdersPerMinute: 0},
		oas.UpdateGeneratorSettingsParams{PointId: quiet.ID})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return onPoint(quiet.ID, deliveryEvents) >= 3 }, waitFor, tick)
	require.Zero(t, onPoint(quiet.ID, orderEvents), "the point has its own rates")
	again, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, products.Total, again.Total)
}

func TestSeedKeepsExistingCatalog(t *testing.T) {
	s := apitest.New(t)
	_, err := s.Client.CreateProduct(ctx, &oas.NewProduct{Name: "Соль", Unit: oas.UnitG}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)

	require.NoError(t, newSimulator(s, s.Client).Seed(ctx, s.CompanyID))

	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.EqualValues(t, 1, products.Total)
	dishes, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Zero(t, dishes.Total)
}

// Владелец отправил в архив блюдо набора: каталог теперь его, генератор не пытается
// создать блюдо заново и не упирается в занятое название.
func TestSeedLeavesACatalogWithArchive(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	dishes, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	_, err = s.Client.ArchiveDish(ctx, oas.ArchiveDishParams{CompanyId: s.CompanyID, DishId: dishes.Items[0].ID})
	require.NoError(t, err)

	require.NoError(t, sim.Seed(ctx, s.CompanyID))

	after, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, dishes.Total-1, after.Total)
}

func TestDeliver(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)

	empty, err := sim.Deliver(ctx, site(s))
	require.NoError(t, err)
	require.Nil(t, empty, "no products, no delivery")

	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	var expired, quarantined int
	for range 40 {
		delivery, err := sim.Deliver(ctx, site(s))
		require.NoError(t, err)

		// состояние проверяется по данным системы, а не по ответу симулятору
		delivery, err = s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: delivery.ID})
		require.NoError(t, err)
		require.Equal(t, oas.DeliveryStatusApproved, delivery.Status)
		require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "dev", Name: oas.NewOptString("dev")}, delivery.ApprovedBy.Value)
		for _, item := range delivery.Items {
			require.Zero(t, item.QuarantinedQuantity, "quarantine is resolved")
			require.Equal(t, item.DeclaredQuantity, item.AcceptedQuantity+item.RejectedQuantity)
			if !item.ExpiresAt.After(s.Clock.Now()) {
				expired++
				require.Zero(t, item.AcceptedQuantity, "an expired item is not accepted")
				require.Empty(t, item.BatchIds)
				require.Equal(t, "expired", item.RejectReason.Value)
			}
			if item.QuarantineReason.Set {
				quarantined++
			}
			if item.AcceptedQuantity > 0 {
				require.NotEmpty(t, item.BatchIds)
			}
		}
	}
	require.Positive(t, expired, "some items are expired on purpose")
	require.Positive(t, quarantined, "some items go through quarantine")
}

func TestOrder(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)

	// блюдо без рецептуры заказать нельзя: заказов нет, ошибки тоже
	_, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: "Черновик"}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	none, err := sim.Order(ctx, site(s))
	require.NoError(t, err)
	require.Nil(t, none)
	require.Zero(t, countEvents(t, s, orderEvents))

	s = apitest.New(t)
	sim = newSimulator(s, s.Client)
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	for range 15 {
		_, err := sim.Deliver(ctx, site(s))
		require.NoError(t, err)
	}

	statuses := map[oas.OrderStatus]int{}
	for range 40 {
		order, err := sim.Order(ctx, site(s))
		require.NoError(t, err)
		statuses[order.Status]++
	}
	require.Positive(t, statuses[oas.OrderStatusAccepted])
	require.Positive(t, statuses[oas.OrderStatusRejected], "oversized orders are rejected")

	var negative int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM inventory_batches WHERE quantity < 0`).Scan(&negative))
	require.Zero(t, negative)
}

func TestRunFollowsSettings(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)
	sim.SeedCatalog = true
	run(t, sim)

	// остановлен: каталог наполнен, действий нет
	require.Eventually(t, func() bool { return sim.Ready(ctx) == nil }, waitFor, tick)
	time.Sleep(settle)
	require.Zero(t, countEvents(t, s, deliveryEvents))
	require.Zero(t, countEvents(t, s, orderEvents))

	// только заказы
	setSettings(t, s, true, 0, 600)
	require.Eventually(t, func() bool { return countEvents(t, s, orderEvents) >= 3 }, waitFor, tick)
	require.Zero(t, countEvents(t, s, deliveryEvents), "zero rate means no deliveries")

	// поставки и заказы
	setSettings(t, s, true, 600, 600)
	require.Eventually(t, func() bool { return countEvents(t, s, deliveryEvents) >= 3 }, waitFor, tick)

	// остановка
	setSettings(t, s, false, 600, 600)
	time.Sleep(settle)
	deliveries, orders := countEvents(t, s, deliveryEvents), countEvents(t, s, orderEvents)
	time.Sleep(settle)
	require.Equal(t, deliveries, countEvents(t, s, deliveryEvents))
	require.Equal(t, orders, countEvents(t, s, orderEvents))
}

// flakyTransport отклоняет запросы, пока включён отказ: так выглядит недоступный API.
type flakyTransport struct {
	down atomic.Bool
}

func (f *flakyTransport) Do(r *http.Request) (*http.Response, error) {
	if f.down.Load() {
		return nil, errors.New("connection refused")
	}
	return http.DefaultClient.Do(r)
}

func TestRunSurvivesAPIOutage(t *testing.T) {
	s := apitest.New(t)
	require.NoError(t, newSimulator(s, s.Client).Seed(ctx, s.CompanyID))
	setSettings(t, s, true, 600, 600)

	transport := &flakyTransport{}
	transport.down.Store(true)
	client, err := oas.NewClient(s.URL+api.Prefix, s.Security(), oas.WithClient(transport))
	require.NoError(t, err)
	sim := newSimulator(s, client)
	run(t, sim)

	time.Sleep(settle)
	require.Error(t, sim.Ready(ctx))
	require.Zero(t, countEvents(t, s, orderEvents))

	// API вернулся: генератор работает дальше
	transport.down.Store(false)
	require.Eventually(t, func() bool { return countEvents(t, s, orderEvents) >= 3 }, waitFor, tick)
	require.NoError(t, sim.Ready(ctx))

	// отказ посреди работы и ещё одно восстановление
	transport.down.Store(true)
	time.Sleep(settle)
	before := countEvents(t, s, orderEvents)
	transport.down.Store(false)
	require.Eventually(t, func() bool { return countEvents(t, s, orderEvents) > before }, waitFor, tick)
}

// Поставщик привозит не только свежее: остаток срока годности у позиций разный.
func TestDeliveriesHaveVariedShelfLife(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	shelfLife := map[string]time.Duration{}
	for _, product := range products.Items {
		shelfLife[product.ID.String()] = time.Duration(product.ShelfLifeDays.Value) * 24 * time.Hour
	}
	now := s.Clock.Now()

	var short, good int
	shares := map[int]bool{} // остаток срока в десятых долях срока хранения
	for range 60 {
		delivery, err := sim.Deliver(ctx, site(s))
		require.NoError(t, err)
		for _, item := range delivery.Items {
			remaining := item.ExpiresAt.Sub(now)
			if remaining <= 0 {
				continue // просроченные позиции проверяет TestDeliver
			}
			good++
			full := shelfLife[item.ProductID.String()]
			require.LessOrEqual(t, remaining, full, "never fresher than the shelf life of the product")
			require.Positive(t, item.AcceptedQuantity, "an item that is still good is accepted")
			if remaining < time.Hour {
				short++
				require.GreaterOrEqual(t, remaining, 3*time.Minute)
				continue
			}
			require.GreaterOrEqual(t, float64(remaining), 0.15*float64(full)-1)
			shares[int(10*float64(remaining)/float64(full))] = true
		}
	}
	require.Positive(t, short, "some batches expire within the hour")
	require.Less(t, short*5, good, "but they stay a rare exception")
	require.GreaterOrEqual(t, len(shares), 6, "the remaining shelf life is spread over the whole range")
}

// На партиях генератора работают горячие предложения: есть блюда с разными приоритетами.
func TestHotOffersUnderGenerator(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	for range 40 {
		_, err := sim.Deliver(ctx, site(s))
		require.NoError(t, err)
	}

	offers, err := s.Client.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.NotEmpty(t, offers.Items)
	priorities := map[int32]bool{}
	for _, offer := range offers.Items {
		priorities[offer.Priority] = true
		require.NotEmpty(t, offer.Reasons)
		require.Positive(t, offer.SuggestedPortions)
	}
	require.Greater(t, len(priorities), 1, "dishes are ranked, not all the same")

	expiring, err := s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.NotEmpty(t, expiring.Items, "the expiring list is not empty either")
}

// Генератор перезапустили посреди наполнения: в каталоге часть набора, блюдо без рецептуры.
// Новый процесс доводит набор до конца, а не считает непустой каталог готовым.
func TestSeedResumesAnUnfinishedSet(t *testing.T) {
	s := apitest.New(t)
	_, err := s.Client.CreateProduct(ctx, &oas.NewProduct{Name: "Молоко", Unit: oas.UnitMl}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	_, err = s.Client.CreateDish(ctx, &oas.NewDish{Name: "Омлет"}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)

	require.NoError(t, newSimulator(s, s.Client).Seed(ctx, s.CompanyID))

	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.EqualValues(t, 11, products.Total)
	dishes, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.EqualValues(t, 5, dishes.Total)
	for _, dish := range dishes.Items {
		require.NotEmpty(t, dish.Ingredients, dish.Name)
	}
}

// Блюдо, в котором есть что-то своё, - каталог владельца: генератор его не дополняет.
func TestSeedKeepsACatalogWithOwnDish(t *testing.T) {
	s := apitest.New(t)
	_, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: "Борщ"}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)

	require.NoError(t, newSimulator(s, s.Client).Seed(ctx, s.CompanyID))

	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Zero(t, products.Total)
}

// Блюдо выключили на точке: генератор его не заказывает, остальные заказы проходят.
func TestOrderSkipsDishesDisabledOnThePoint(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, s.Client)
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	dishes, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	off := map[uuid.UUID]bool{}
	for _, dish := range dishes.Items[:len(dishes.Items)-1] {
		off[dish.ID] = true
		_, err := s.Client.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: false}, oas.SetDishOnPointParams{PointId: s.PointID, DishId: dish.ID})
		require.NoError(t, err)
	}

	for range 20 {
		order, err := sim.Order(ctx, site(s))
		require.NoError(t, err)
		require.NotNil(t, order)
		for _, item := range order.Items {
			require.False(t, off[item.DishID])
		}
	}

	// все блюда выключены: заказов нет, ошибок тоже
	last := dishes.Items[len(dishes.Items)-1]
	_, err = s.Client.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: false}, oas.SetDishOnPointParams{PointId: s.PointID, DishId: last.ID})
	require.NoError(t, err)
	none, err := sim.Order(ctx, site(s))
	require.NoError(t, err)
	require.Nil(t, none)
}

// failingAPI отклоняет первую приёмку: так выглядит сбой посреди поставки.
type failingAPI struct {
	generator.API
	failed bool
}

func (a *failingAPI) ApproveDelivery(ctx context.Context, request *oas.DeliveryApproval, params oas.ApproveDeliveryParams) (*oas.Delivery, error) {
	if !a.failed {
		a.failed = true
		return nil, errors.New("connection reset")
	}
	return a.API.ApproveDelivery(ctx, request, params)
}

// Поставка сорвалась после регистрации: генератор доводит её до конца, а не оставляет без приёмки.
// Поставку сотрудника с обычным номером он при этом не трогает.
func TestResumeFinishesInterruptedDeliveries(t *testing.T) {
	s := apitest.New(t)
	sim := newSimulator(s, &failingAPI{API: s.Client})
	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	staff, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-1", DeliveredAt: s.Clock.Now(),
		Items: []oas.NewDeliveryItem{{ProductID: products.Items[0].ID, DeclaredQuantity: 10, ExpiresAt: s.Clock.Now().Add(48 * time.Hour)}},
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)

	_, err = sim.Deliver(ctx, site(s))
	require.Error(t, err)
	registered := func() []oas.Delivery {
		s.Sync(t)
		page, err := s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{
			PointId: s.PointID, Status: oas.NewOptListDeliveriesStatus(oas.ListDeliveriesStatusRegistered),
		})
		require.NoError(t, err)
		return page.Items
	}
	require.Len(t, registered(), 2, "the interrupted delivery is left without approval")

	require.NoError(t, sim.Resume(ctx, site(s)))
	left := registered()
	require.Len(t, left, 1)
	require.Equal(t, staff.ID, left[0].ID, "the delivery of the staff is not touched")
	open, err := s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{
		PointId: s.PointID, Quarantine: oas.NewOptListDeliveriesQuarantine(oas.ListDeliveriesQuarantineOpen),
	})
	require.NoError(t, err)
	require.Empty(t, open.Items, "no quarantine is left open")
}

// Весь путь симулятора под его сервисной учётной записью: у неё только права генератора на точке,
// где он включён. Остальные тесты идут от владельца и нехватку права не заметили бы.
func TestSimulatorWorksWithGeneratorPermissionsOnly(t *testing.T) {
	s := apitest.New(t)
	setSettings(t, s, true, 6, 30)
	account := s.As(t, eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"}, "generator.run")
	sim := newSimulator(s, &failingAPI{API: account})

	require.NoError(t, sim.Seed(ctx, s.CompanyID))
	_, err := sim.Deliver(ctx, site(s))
	require.Error(t, err, "the first approval is rejected by the test")
	s.Sync(t)
	require.NoError(t, sim.Resume(ctx, site(s)))
	for range 5 {
		delivery, err := sim.Deliver(ctx, site(s))
		require.NoError(t, err)
		require.NotNil(t, delivery)
	}
	order, err := sim.Order(ctx, site(s))
	require.NoError(t, err)
	require.NotNil(t, order)
}
