package generator

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/observability"
)

// Доли намеренно проблемных случаев. Это свойства сценария, а не режим работы:
// настройками API они не управляются.
const (
	expiredItemShare     = 0.10 // позиция поставки уже просрочена
	shortLifeItemShare   = 0.05 // до конца срока годности остаются минуты
	quarantinedItemShare = 0.15 // часть годной позиции уходит в карантин
	oversizedOrderShare  = 0.10 // заказ заведомо больше остатков
	// Порций в заведомо завышенном заказе - наибольшее, что принимает API. Такой заказ отклоняется
	// при любом правдоподобном остатке. Прежние 200-500 порций при большом остатке принимались
	// и съедали сотни килограммов: остатки, расход и рекомендуемые поставки выглядели неправдоподобно.
	oversizedPortions = 10_000

	// Остаток срока годной позиции - случайная часть срока хранения: поставщик привозит не только
	// свежее. Так на складе есть партии с разным сроком, и работают горячие предложения и список
	// истекающих партий. Нижняя граница оставляет хлебу (3 дня) не меньше 10 часов.
	minRemainingShare = 0.15
	// Позиция с коротким сроком успевает получить предупреждение и быть списанной за время демо.
	shortLifeMin, shortLifeMax = 3 * time.Minute, 20 * time.Minute

	defaultShelfLife = 7 * 24 * time.Hour // для продуктов без срока хранения в каталоге
	pageSize         = 200                // каталог читается одной страницей
	maxPerTick       = 20                 // чтобы после простоя не выполнить всё накопленное разом
	actionTimeout    = 15 * time.Second
	// generatedNumber - начало номера поставок генератора: по нему он отличает свои поставки
	// от поставок сотрудников, когда доводит незаконченные до конца.
	generatedNumber = "ГЕН-"
)

// API - операции публичного API, которыми пользуется симулятор. Реализует *oas.Client.
// Других способов изменить систему у симулятора нет.
type API interface {
	ListGeneratorPoints(ctx context.Context) ([]oas.GeneratorPoint, error)
	ListProducts(ctx context.Context, params oas.ListProductsParams) (*oas.ProductPage, error)
	ListDishes(ctx context.Context, params oas.ListDishesParams) (*oas.DishPage, error)
	ListPointDishes(ctx context.Context, params oas.ListPointDishesParams) (*oas.PointDishPage, error)
	CreateProduct(ctx context.Context, request *oas.NewProduct, params oas.CreateProductParams) (*oas.Product, error)
	CreateDish(ctx context.Context, request *oas.NewDish, params oas.CreateDishParams) (*oas.Dish, error)
	ChangeRecipe(ctx context.Context, request *oas.Recipe, params oas.ChangeRecipeParams) (*oas.Dish, error)
	ListDeliveries(ctx context.Context, params oas.ListDeliveriesParams) (*oas.DeliveryPage, error)
	GetDelivery(ctx context.Context, params oas.GetDeliveryParams) (*oas.Delivery, error)
	RegisterDelivery(ctx context.Context, request *oas.NewDelivery, params oas.RegisterDeliveryParams) (*oas.Delivery, error)
	ApproveDelivery(ctx context.Context, request *oas.DeliveryApproval, params oas.ApproveDeliveryParams) (*oas.Delivery, error)
	ResolveQuarantine(ctx context.Context, request *oas.QuarantineResolution, params oas.ResolveQuarantineParams) (*oas.Delivery, error)
	PlaceOrder(ctx context.Context, request *oas.NewOrder, params oas.PlaceOrderParams) (*oas.Order, error)
}

// Simulator эмулирует работу ресторана запросами к публичному API.
type Simulator struct {
	api    API
	clock  clock.Clock
	rng    *rand.Rand
	logger *slog.Logger
	// SeedCatalog включает наполнение пустого каталога демонстрационным набором при первом удачном опросе.
	SeedCatalog bool

	actions *prometheus.CounterVec
	orders  *prometheus.CounterVec

	ready atomic.Bool
	// компании, каталог которых уже наполнен или был непуст: наполнение проверяется один раз
	seeded map[uuid.UUID]bool
}

// Site - точка, на которой работает генератор, и её компания: поставки и заказы создаются
// на точке, продукты и блюда берутся из каталога компании.
type Site struct {
	PointID   uuid.UUID
	CompanyID uuid.UUID
}

// running - состояние одной точки с включённым генератором: интенсивность и накопленные действия.
type running struct {
	settings           oas.GeneratorPoint
	deliveries, orders accumulator
	// resume - на точке могли остаться незаконченные поставки генератора: точка только что
	// появилась у этого процесса или поставка на ней сорвалась.
	resume bool
}

// NewSimulator создаёт симулятор. Источник случайности передаётся снаружи: тестам нужна воспроизводимость.
func NewSimulator(api API, clk clock.Clock, rng *rand.Rand, logger *slog.Logger) *Simulator {
	return &Simulator{
		api: api, clock: clk, rng: rng, logger: logger,
		seeded: map[uuid.UUID]bool{},
		actions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "generator_actions_total",
			Help: "Generator actions by kind (delivery, order) and outcome (done, error).",
		}, []string{"action", "outcome"}),
		orders: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "generator_orders_total",
			Help: "Orders placed by the generator by status.",
		}, []string{"status"}),
	}
}

// Collectors возвращает метрики симулятора для регистрации в реестре процесса.
func (s *Simulator) Collectors() []prometheus.Collector {
	return []prometheus.Collector{s.actions, s.orders}
}

// count учитывает исход действия; действие, которому нечего делать (пустой каталог), не считается.
func (s *Simulator) count(action string, performed bool, err error) {
	switch {
	case err != nil:
		s.actions.WithLabelValues(action, "error").Inc()
	case performed:
		s.actions.WithLabelValues(action, "done").Inc()
	}
}

// Ready сообщает, удался ли последний опрос точек через API.
func (s *Simulator) Ready(context.Context) error {
	if !s.ready.Load() {
		return errors.New("generator: api is not reachable")
	}
	return nil
}

// Run раз в poll опрашивает точки, на которых генератор включён, и раз в tick выполняет на каждой
// накопившиеся поставки и заказы с её интенсивностью. Работает до отмены контекста.
// Ошибки отдельных действий не останавливают цикл.
func (s *Simulator) Run(ctx context.Context, poll, tick time.Duration) error {
	pollTicker, tickTicker := time.NewTicker(poll), time.NewTicker(tick)
	defer pollTicker.Stop()
	defer tickTicker.Stop()

	// до первого удачного опроса генератор нигде не работает
	points := map[uuid.UUID]*running{}
	refresh := func() {
		current, err := s.api.ListGeneratorPoints(ctx)
		if err != nil {
			s.ready.Store(false)
			s.logger.WarnContext(ctx, "generator: cannot read points", "error", err)
			return
		}
		s.ready.Store(true)
		points = s.reconcile(ctx, points, current)
	}
	refresh()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-pollTicker.C:
			refresh()
		case <-tickTicker.C:
			for _, point := range points {
				site := Site{PointID: point.settings.PointID, CompanyID: point.settings.CompanyID}
				if point.resume {
					s.act(ctx, "resume", func(ctx context.Context) error {
						err := s.Resume(ctx, site)
						point.resume = err != nil
						return err
					})
				}
				for range point.deliveries.due(point.settings.DeliveriesPerMinute, tick) {
					s.act(ctx, "delivery", func(ctx context.Context) error {
						_, err := s.Deliver(ctx, site)
						point.resume = point.resume || err != nil
						return err
					})
				}
				for range point.orders.due(point.settings.OrdersPerMinute, tick) {
					s.act(ctx, "order", func(ctx context.Context) error {
						_, err := s.Order(ctx, site)
						return err
					})
				}
			}
		}
	}
}

// reconcile приводит состояние точек к списку включённых: у оставшихся сохраняет накопленные
// действия, у новых начинает счёт с нуля, выключенные забывает - они не копят действия
// на момент следующего включения. Пустой каталог компании новой точки наполняется.
func (s *Simulator) reconcile(ctx context.Context, known map[uuid.UUID]*running, current []oas.GeneratorPoint) map[uuid.UUID]*running {
	points := make(map[uuid.UUID]*running, len(current))
	for _, settings := range current {
		if s.SeedCatalog && !s.seeded[settings.CompanyID] {
			if err := s.Seed(ctx, settings.CompanyID); err != nil {
				s.logger.WarnContext(ctx, "generator: cannot seed catalog", "company_id", settings.CompanyID, "error", err)
				continue // без каталога на точке делать нечего, попытка повторится при следующем опросе
			}
			s.seeded[settings.CompanyID] = true
		}
		point, found := known[settings.PointID]
		if !found {
			point = &running{resume: true}
		}
		point.settings = settings
		points[settings.PointID] = point
	}
	return points
}

// act выполняет одно действие с таймаутом. Ошибка пишется в лог и не прерывает работу:
// следующее действие наступит через долю секунды.
func (s *Simulator) act(ctx context.Context, name string, do func(ctx context.Context) error) {
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	// молчим только при остановке процесса: истечение собственного срока действия - тоже сбой
	if err := do(ctx); err != nil && parent.Err() == nil {
		s.logger.WarnContext(ctx, "generator: action failed", "action", name, "error", err)
	}
}

// accumulator считает, сколько действий пора выполнить при заданной интенсивности.
// Счёт идёт в целых числах, поэтому дробные остатки не теряются и не накапливают погрешность.
type accumulator struct {
	pending int64 // в единицах "действие * наносекунда в минуте"
}

func (a *accumulator) due(perMinute int32, tick time.Duration) int {
	a.pending += int64(perMinute) * tick.Nanoseconds()
	n := a.pending / int64(time.Minute)
	a.pending %= int64(time.Minute)
	if n > maxPerTick {
		return maxPerTick
	}
	return int(n)
}

// Deliver создаёт одну поставку по полному пути из ADR-012: регистрация, приёмка,
// решение по своему карантину - на точке site. Возвращает nil, если в каталоге её компании нет продуктов.
func (s *Simulator) Deliver(ctx context.Context, site Site) (delivery *oas.Delivery, err error) {
	defer func() { s.count("delivery", delivery != nil, err) }()
	companyID, pointID := site.CompanyID, site.PointID
	products, err := s.api.ListProducts(ctx, oas.ListProductsParams{CompanyId: companyID, Limit: oas.NewOptInt32(pageSize)})
	if err != nil {
		return nil, err
	}
	if len(products.Items) == 0 {
		return nil, nil
	}
	now := s.clock.Now()

	picked := pick(s.rng, products.Items, 1+s.rng.IntN(3))
	request := &oas.NewDelivery{
		Number: generatedNumber + now.Format("20060102-150405.000"), DeliveredAt: now,
		Items: make([]oas.NewDeliveryItem, len(picked)),
	}
	for i, product := range picked {
		expiresAt := now.Add(s.remainingLife(product))
		// Дата изготовления не передаётся: проверка согласованности дат со сроком хранения
		// отклонила бы намеренно просроченную позицию ещё при регистрации.
		request.Items[i] = oas.NewDeliveryItem{
			ProductID: product.ID, DeclaredQuantity: s.quantity(product.Unit), ExpiresAt: expiresAt,
		}
	}
	delivery, err = s.api.RegisterDelivery(ctx, request, oas.RegisterDeliveryParams{PointId: pointID, IdempotencyKey: newKey()})
	if err != nil {
		return nil, err
	}

	return s.finish(ctx, pointID, delivery)
}

// finish доводит поставку до конца: приёмка, если её ещё не было, и решение по своему карантину.
func (s *Simulator) finish(ctx context.Context, pointID uuid.UUID, delivery *oas.Delivery) (*oas.Delivery, error) {
	now := s.clock.Now()
	var err error
	if delivery.Status == oas.DeliveryStatusRegistered {
		delivery, err = s.approve(ctx, pointID, delivery, now)
		if err != nil {
			return nil, err
		}
	}

	// карантин не должен остаться без решения
	for _, item := range delivery.Items {
		if item.QuarantinedQuantity == 0 {
			continue
		}
		resolution := &oas.QuarantineResolution{AcceptedQuantity: item.QuarantinedQuantity}
		if s.rng.IntN(2) == 0 {
			resolution = &oas.QuarantineResolution{
				RejectedQuantity: item.QuarantinedQuantity,
				RejectReason:     oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation),
				Comment:          oas.NewOptString("температура выше допустимой"),
			}
		}
		delivery, err = s.api.ResolveQuarantine(ctx, resolution, oas.ResolveQuarantineParams{
			PointId:    pointID,
			DeliveryId: delivery.ID, ItemId: item.ID, IdempotencyKey: newKey(),
		})
		if err != nil {
			return nil, err
		}
	}
	return delivery, nil
}

func (s *Simulator) approve(ctx context.Context, pointID uuid.UUID, delivery *oas.Delivery, now time.Time) (*oas.Delivery, error) {
	approval := &oas.DeliveryApproval{Items: make([]oas.ItemDecision, len(delivery.Items))}
	for i, item := range delivery.Items {
		decision := oas.ItemDecision{ItemID: item.ID}
		switch {
		case !item.ExpiresAt.After(now):
			decision.RejectedQuantity = item.DeclaredQuantity
			decision.RejectReason = oas.NewOptWriteOffReason(oas.WriteOffReasonExpired)
			decision.Comment = oas.NewOptString("срок годности истёк до приёмки")
		case s.rng.Float64() < quarantinedItemShare:
			decision.QuarantinedQuantity = max(item.DeclaredQuantity/5, 1)
			decision.AcceptedQuantity = item.DeclaredQuantity - decision.QuarantinedQuantity
			decision.QuarantineReason = oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation)
			decision.Comment = oas.NewOptString("проверка температуры при перевозке")
		default:
			decision.AcceptedQuantity = item.DeclaredQuantity
		}
		approval.Items[i] = decision
	}
	return s.api.ApproveDelivery(ctx, approval, oas.ApproveDeliveryParams{PointId: pointID, DeliveryId: delivery.ID, IdempotencyKey: newKey()})
}

// Resume доводит до конца поставки генератора на точке, оставшиеся без приёмки или с открытым
// карантином: так бывает, когда запрос посреди поставки не прошёл или процесс остановили.
// Поставки сотрудников (номер без generatedNumber) не трогает.
func (s *Simulator) Resume(ctx context.Context, site Site) error {
	for _, params := range []oas.ListDeliveriesParams{
		{PointId: site.PointID, Status: oas.NewOptListDeliveriesStatus(oas.ListDeliveriesStatusRegistered), Limit: oas.NewOptInt32(pageSize)},
		{PointId: site.PointID, Quarantine: oas.NewOptListDeliveriesQuarantine(oas.ListDeliveriesQuarantineOpen), Limit: oas.NewOptInt32(pageSize)},
	} {
		page, err := s.api.ListDeliveries(ctx, params)
		if err != nil {
			return err
		}
		for _, listed := range page.Items {
			if !strings.HasPrefix(listed.Number, generatedNumber) {
				continue
			}
			// список - проекция и может отставать: состояние берётся из самой поставки
			delivery, err := s.api.GetDelivery(ctx, oas.GetDeliveryParams{PointId: site.PointID, DeliveryId: listed.ID})
			if err != nil {
				return err
			}
			if _, err := s.finish(ctx, site.PointID, delivery); err != nil {
				return err
			}
		}
	}
	return nil
}

// Order оформляет один заказ. Отклонённый заказ - не ошибка: часть заказов намеренно
// превышает остатки. Возвращает nil, если на точке не продаётся ни одного блюда с рецептурой.
func (s *Simulator) Order(ctx context.Context, site Site) (order *oas.Order, err error) {
	defer func() { s.count("order", order != nil, err) }()
	companyID, pointID := site.CompanyID, site.PointID
	dishes, err := s.api.ListDishes(ctx, oas.ListDishesParams{CompanyId: companyID, Limit: oas.NewOptInt32(pageSize)})
	if err != nil {
		return nil, err
	}
	// Блюдо, выключенное на точке, не заказывается: заказ с ним отклоняется целиком без события.
	onPoint, err := s.api.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: pointID, Limit: oas.NewOptInt32(pageSize)})
	if err != nil {
		return nil, err
	}
	sold := make(map[uuid.UUID]bool, len(onPoint.Items))
	for _, dish := range onPoint.Items {
		sold[dish.DishID] = dish.Enabled
	}
	var cookable []oas.Dish
	for _, dish := range dishes.Items {
		if len(dish.Ingredients) > 0 && sold[dish.ID] {
			cookable = append(cookable, dish)
		}
	}
	if len(cookable) == 0 {
		return nil, nil
	}

	picked := pick(s.rng, cookable, 1+s.rng.IntN(2))
	oversized := s.rng.Float64() < oversizedOrderShare
	request := &oas.NewOrder{Items: make([]oas.OrderItem, len(picked))}
	for i, dish := range picked {
		portions := int64(1 + s.rng.IntN(3))
		if oversized {
			portions = oversizedPortions
		}
		request.Items[i] = oas.OrderItem{DishID: dish.ID, Portions: portions}
	}

	order, err = s.api.PlaceOrder(ctx, request, oas.PlaceOrderParams{PointId: pointID, IdempotencyKey: newKey()})
	if err != nil {
		return nil, err
	}
	s.orders.WithLabelValues(string(order.Status)).Inc()
	s.logger.DebugContext(ctx, "generator: order placed", "order_id", order.ID, "status", order.Status)
	return order, nil
}

// quantity выбирает количество в единице продукта: для граммов и миллилитров - тысячи, для штук - десятки.
func (s *Simulator) quantity(unit oas.Unit) int64 {
	if unit == oas.UnitPcs {
		return int64(10 + s.rng.IntN(41))
	}
	return int64(1000 + s.rng.IntN(4001))
}

// remainingLife выбирает, сколько осталось до конца срока годности позиции. Отрицательное
// значение - позиция уже просрочена, генератор отклонит её при приёмке.
func (s *Simulator) remainingLife(product oas.Product) time.Duration {
	switch roll := s.rng.Float64(); {
	case roll < expiredItemShare:
		return -time.Duration(1+s.rng.IntN(48)) * time.Hour
	case roll < expiredItemShare+shortLifeItemShare:
		return shortLifeMin + time.Duration(s.rng.Int64N(int64(shortLifeMax-shortLifeMin)))
	default:
		share := minRemainingShare + s.rng.Float64()*(1-minRemainingShare)
		return time.Duration(float64(shelfLife(product)) * share)
	}
}

func shelfLife(product oas.Product) time.Duration {
	if days, set := product.ShelfLifeDays.Get(); set {
		return time.Duration(days) * 24 * time.Hour
	}
	return defaultShelfLife
}

// pick возвращает n разных случайных элементов; если элементов меньше, то все.
func pick[T any](rng *rand.Rand, items []T, n int) []T {
	n = min(n, len(items))
	picked := make([]T, 0, n)
	for _, i := range rng.Perm(len(items))[:n] {
		picked = append(picked, items[i])
	}
	return picked
}

// newKey - ключ идемпотентности запроса на запись.
func newKey() oas.OptString {
	return oas.NewOptString(uuid.NewString())
}
