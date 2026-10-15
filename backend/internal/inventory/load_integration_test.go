//go:build integration

package inventory_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	simulator "goden/backend/internal/generator"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/platform/eventstore"
)

const (
	loadClients       = 4   // параллельные клиенты, создающие поставки и заказы
	defaultLoadOrders = 300 // GODEN_LOAD_ORDERS задаёт другое число для ручного прогона
	ordersPerDelivery = 6
)

// expected пропускает ответы, которые при гонке за остаток закономерны: партию мог опустошить
// заказ, пока клиент выбирал количество. Внутренняя ошибка и сбой транспорта - всегда провал.
func expected(err error, statuses ...int) error {
	var apiErr *oas.ErrorStatusCode
	if errors.As(err, &apiErr) {
		for _, status := range statuses {
			if apiErr.StatusCode == status {
				return nil
			}
		}
		return fmt.Errorf("unexpected status %d: %s", apiErr.StatusCode, apiErr.Response.Detail.Value)
	}
	return err
}

func isStatus(err error, status int) bool {
	var apiErr *oas.ErrorStatusCode
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}

// site - точка пользователя dev: на ней симулятор создаёт поставки и заказы в тестах.
func site(s apitest.Server) simulator.Site {
	return simulator.Site{PointID: s.PointID, CompanyID: s.CompanyID}
}

func loadSimulator(s apitest.Server, seed uint64) *simulator.Simulator {
	return simulator.NewSimulator(s.Client, s.Clock, rand.New(rand.NewPCG(seed, 7)), slog.New(slog.DiscardHandler))
}

// Несколько клиентов одновременно создают поставки, оформляют заказы, списывают и корректируют
// остатки, а рецептура блюда в это время меняется. Числа зависят от исхода гонок, поэтому
// проверяются свойства, которые обязаны выполняться при любом исходе.
func TestStockStaysConsistentUnderLoad(t *testing.T) {
	total := defaultLoadOrders
	if v, err := strconv.Atoi(os.Getenv("GODEN_LOAD_ORDERS")); err == nil && v > 0 {
		total = v
	}
	s := apitest.New(t)
	require.NoError(t, loadSimulator(s, 0).Seed(ctx, s.CompanyID))
	warmUp := loadSimulator(s, 1)
	for range 25 {
		_, err := warmUp.Deliver(ctx, site(s))
		require.NoError(t, err)
	}
	dishes, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	products, err := s.Client.ListProducts(ctx, oas.ListProductsParams{CompanyId: s.CompanyID})
	require.NoError(t, err)

	var (
		remaining            atomic.Int64
		stop                 atomic.Bool
		accepted, rejected   atomic.Int64
		writeOffs, adjusted  atomic.Int64
		recipeChanges, moved atomic.Int64
		conflicts            atomic.Int64
	)
	remaining.Store(int64(total))
	started := time.Now()

	var clients, background errgroup.Group
	for client := range loadClients {
		clients.Go(func() error {
			sim := loadSimulator(s, uint64(client+10))
			for n := 0; remaining.Add(-1) >= 0; n++ {
				if n%ordersPerDelivery == 0 {
					if _, err := sim.Deliver(ctx, site(s)); err != nil {
						return fmt.Errorf("deliver: %w", err)
					}
					moved.Add(1)
				}
				order, err := sim.Order(ctx, site(s))
				if isStatus(err, http.StatusConflict) {
					// Рецептура менялась быстрее, чем заказ успевал взять блокировки под новый состав:
					// команда исчерпала повторы и просит клиента повторить. Заказ не оформлен.
					conflicts.Add(1)
					remaining.Add(1)
					continue
				}
				if err != nil {
					return fmt.Errorf("order: %w", err)
				}
				if order.Status == oas.OrderStatusAccepted {
					accepted.Add(1)
				} else {
					rejected.Add(1)
				}
			}
			return nil
		})
	}

	// списания и корректировки случайных партий
	background.Go(func() error {
		rng := rand.New(rand.NewPCG(99, 7))
		for !stop.Load() {
			product := products.Items[rng.IntN(len(products.Items))]
			batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: product.ID})
			if err != nil {
				return fmt.Errorf("list batches: %w", err)
			}
			if len(batches.Items) == 0 {
				continue
			}
			batch := batches.Items[rng.IntN(len(batches.Items))]
			if rng.IntN(2) == 0 {
				_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
					Quantity: 1 + batch.Quantity/20, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "нагрузка",
				}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batch.ID})
				writeOffs.Add(1)
			} else {
				_, err = s.Client.AdjustStock(ctx, &oas.Adjustment{
					Delta: int64(rng.IntN(21)) - 10, Reason: "inventory_count",
				}, oas.AdjustStockParams{PointId: s.PointID, BatchId: batch.ID})
				adjusted.Add(1)
			}
			// 422: количество больше остатка или нулевая корректировка
			if err := expected(err, http.StatusUnprocessableEntity); err != nil {
				return fmt.Errorf("stock change: %w", err)
			}
		}
		return nil
	})

	// рецептура одного блюда меняется между двумя вариантами: заказы перечитывают её под блокировками
	background.Go(func() error {
		dish := dishes.Items[0]
		recipe := make([]oas.Ingredient, len(dish.Ingredients))
		for i, ingredient := range dish.Ingredients {
			recipe[i] = oas.Ingredient{ProductID: ingredient.ProductID, Quantity: ingredient.Quantity}
		}
		variants := [][]oas.Ingredient{recipe, recipe[:len(recipe)-1]}
		for n := 1; !stop.Load(); n++ {
			_, err := s.Client.ChangeRecipe(ctx, &oas.Recipe{Ingredients: variants[n%2]}, oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
			if err != nil {
				return fmt.Errorf("change recipe: %w", err)
			}
			recipeChanges.Add(1)
			time.Sleep(5 * time.Millisecond)
		}
		return nil
	})

	clientsErr := clients.Wait()
	stop.Store(true)
	require.NoError(t, background.Wait())
	require.NoError(t, clientsErr)
	elapsed := time.Since(started)

	placed := accepted.Load() + rejected.Load()
	operations := placed + moved.Load() + writeOffs.Load() + adjusted.Load() + recipeChanges.Load()
	t.Logf("orders %d (accepted %d, rejected %d, asked to retry %d), deliveries %d, write-offs %d, adjustments %d, recipe changes %d; %d operations in %s, %.0f per second",
		placed, accepted.Load(), rejected.Load(), conflicts.Load(), moved.Load(), writeOffs.Load(), adjusted.Load(), recipeChanges.Load(),
		operations, elapsed.Round(time.Millisecond), float64(operations)/elapsed.Seconds())
	require.EqualValues(t, total, placed)
	require.Positive(t, accepted.Load())
	require.Positive(t, rejected.Load())
	require.Positive(t, recipeChanges.Load())

	requireStockMatchesEvents(t, s)
	requireOrdersConsumedByRecipe(t, s)
	requireStockReplays(t, s)
}

// requireStockMatchesEvents считает остаток каждой партии по событиям отдельным запросом,
// без функций apply*, и сравнивает с таблицей.
func requireStockMatchesEvents(t *testing.T, s apitest.Server) {
	t.Helper()
	var batches, received, mismatched, negative int
	require.NoError(t, s.Pool.QueryRow(ctx, `
		WITH received AS (
			SELECT (payload->>'batch_id')::uuid AS id, (payload->>'quantity')::bigint AS quantity
			FROM events WHERE event_type = 'BatchReceived'
		), moves AS (
			SELECT (b->>'batch_id')::uuid AS id, -(b->>'quantity')::bigint AS delta
			FROM events, jsonb_array_elements(payload->'batches') AS b WHERE event_type = 'StockConsumed'
			UNION ALL
			SELECT (payload->>'batch_id')::uuid, -(payload->>'quantity')::bigint FROM events WHERE event_type = 'StockWrittenOff'
			UNION ALL
			SELECT (payload->>'batch_id')::uuid, (payload->>'delta')::bigint FROM events WHERE event_type = 'StockAdjusted'
		), expected AS (
			SELECT r.id, r.quantity AS received, r.quantity + coalesce(sum(m.delta), 0) AS quantity
			FROM received r LEFT JOIN moves m USING (id) GROUP BY r.id, r.quantity
		)
		SELECT (SELECT count(*) FROM inventory_batches),
		       (SELECT count(*) FROM expected),
		       (SELECT count(*) FROM inventory_batches b FULL JOIN expected e USING (id)
		        WHERE b.quantity IS DISTINCT FROM e.quantity OR b.received_quantity IS DISTINCT FROM e.received),
		       (SELECT count(*) FROM inventory_batches WHERE quantity < 0)`,
	).Scan(&batches, &received, &mismatched, &negative))
	require.Positive(t, batches)
	require.Equal(t, received, batches, "one row per received batch")
	require.Zero(t, mismatched, "batch quantity = received - consumed - written off + adjusted")
	require.Zero(t, negative)
}

// requireOrdersConsumedByRecipe читает события в общем порядке, как их увидят проекции, и проверяет:
// версии каждого потока идут подряд; принятый заказ израсходовал ровно потребность по рецептуре;
// отклонённый не израсходовал ничего.
func requireOrdersConsumedByRecipe(t *testing.T, s apitest.Server) {
	t.Helper()
	type recipe = map[uuid.UUID]int64
	var (
		versions                                         = map[string]int64{}
		current                                          = map[uuid.UUID]recipe{} // рецептура блюда на текущий момент чтения
		previous                                         = map[uuid.UUID]recipe{} // и предыдущая
		consumed                                         = map[uuid.UUID]recipe{} // расход по заказам
		acceptedOrders, rejectedOrders, byPreviousRecipe int
		after                                            eventstore.Checkpoint
	)
	needs := func(items []orders.OrderItem, recipes func(dish uuid.UUID) recipe) recipe {
		need := recipe{}
		for _, item := range items {
			for product, quantity := range recipes(item.DishID) {
				need[product] += quantity * item.Portions
			}
		}
		return need
	}

	for {
		events, err := eventstore.ReadAll(ctx, s.Pool, after, 1000)
		require.NoError(t, err)
		if len(events) == 0 {
			break
		}
		for _, e := range events {
			after = e.Checkpoint()
			require.Equal(t, versions[e.StreamID]+1, e.Version, "stream %s is read in version order", e.StreamID)
			versions[e.StreamID] = e.Version

			payload, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
			require.NoError(t, err)
			switch p := payload.(type) {
			case catalog.RecipeChanged:
				ingredients := recipe{}
				for _, ingredient := range p.Ingredients {
					ingredients[ingredient.ProductID] = ingredient.Quantity
				}
				previous[p.DishID], current[p.DishID] = current[p.DishID], ingredients
			case inventory.StockConsumed:
				if consumed[p.OrderID] == nil {
					consumed[p.OrderID] = recipe{}
				}
				for _, batch := range p.Batches {
					consumed[p.OrderID][p.ProductID] += batch.Quantity
				}
			case orders.OrderAccepted:
				acceptedOrders++
				got := consumed[p.OrderID]
				if maps.Equal(needs(p.Items, func(dish uuid.UUID) recipe { return current[dish] }), got) {
					continue
				}
				// Заказ читает рецептуру в своей транзакции, а место в общем порядке получает по номеру
				// транзакции. Изменение рецептуры, зафиксированное рядом с заказом, может оказаться
				// в общем порядке по другую сторону от него, чем видел заказ. Вариантов рецептуры в тесте
				// два, поэтому "соседняя" версия - это предыдущая.
				details, _ := json.Marshal(map[string]any{"order": p, "consumed": got})
				require.True(t, maps.Equal(needs(p.Items, func(dish uuid.UUID) recipe { return previous[dish] }), got) ||
					mixedRecipes(p.Items, current, previous, got),
					"order consumed neither by the current nor by the previous recipe: %s", details)
				byPreviousRecipe++
			case orders.OrderRejected:
				rejectedOrders++
				require.Empty(t, consumed[p.OrderID], "a rejected order consumes nothing")
			}
		}
	}
	require.Positive(t, acceptedOrders)
	require.Positive(t, rejectedOrders)
	t.Logf("accepted orders %d, of them consumed by the neighbouring version of a recipe changed concurrently: %d", acceptedOrders, byPreviousRecipe)
}

// mixedRecipes: в заказе два блюда, и только одно из них попало под изменение рецептуры.
func mixedRecipes(items []orders.OrderItem, current, previous map[uuid.UUID]map[uuid.UUID]int64, got map[uuid.UUID]int64) bool {
	for _, old := range items {
		need := map[uuid.UUID]int64{}
		for _, item := range items {
			recipes := current
			if item.DishID == old.DishID {
				recipes = previous
			}
			for product, quantity := range recipes[item.DishID] {
				need[product] += quantity * item.Portions
			}
		}
		if maps.Equal(need, got) {
			return true
		}
	}
	return false
}

// Параллельные заказы вместе требуют больше продукта, чем есть: лишние отклоняются.
func TestConcurrentOrdersThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream := apiProduct(t, s, "Сливки", oas.UnitMl)
	pasta := apiDish(t, s, "Паста", oas.Ingredient{ProductID: cream.ID, Quantity: 80})
	apiSupply(t, s, cream, 300, oct(12))
	apiSupply(t, s, cream, 200, oct(15))

	var accepted, rejected atomic.Int32
	var g errgroup.Group
	for range 12 {
		g.Go(func() error {
			order, err := s.Client.PlaceOrder(ctx,
				&oas.NewOrder{Items: []oas.OrderItem{{DishID: pasta.ID, Portions: 1}}}, oas.PlaceOrderParams{PointId: s.PointID})
			if err != nil {
				return err
			}
			if order.Status == oas.OrderStatusAccepted {
				accepted.Add(1)
			} else {
				rejected.Add(1)
			}
			return nil
		})
	}
	require.NoError(t, g.Wait())

	require.EqualValues(t, 6, accepted.Load(), "500 / 80")
	require.EqualValues(t, 6, rejected.Load())
	require.Equal(t, []int64{0, 20}, apiQuantities(t, s, cream))
	requireStockMatchesEvents(t, s)
	requireOrdersConsumedByRecipe(t, s)
	requireStockReplays(t, s)
}
