//go:build integration

package api_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func dishWith(t *testing.T, s apitest.Server, name string, ingredients ...oas.Ingredient) *oas.Dish {
	t.Helper()
	dish := createDish(t, s, name)
	dish, err := s.Client.ChangeRecipe(ctx, recipe(ingredients...), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	return dish
}

func menu(t *testing.T, s apitest.Server) map[string]oas.DishAvailability {
	t.Helper()
	page, err := s.Client.GetMenu(ctx, oas.GetMenuParams{PointId: s.PointID})
	require.NoError(t, err)
	out := make(map[string]oas.DishAvailability, len(page.Items))
	for _, item := range page.Items {
		out[item.Dish.Name] = item
	}
	return out
}

// stopped возвращает названия блюд стоп-листа в порядке ответа.
func stopped(t *testing.T, s apitest.Server) []string {
	t.Helper()
	page, err := s.Client.GetStopList(ctx, oas.GetStopListParams{PointId: s.PointID})
	require.NoError(t, err)
	names := make([]string, len(page.Items))
	for i, item := range page.Items {
		require.False(t, item.Available)
		require.Zero(t, item.Portions)
		names[i] = item.Dish.Name
	}
	require.EqualValues(t, len(names), page.Total)
	return names
}

func limitingProducts(a oas.DishAvailability) []string {
	names := make([]string, len(a.Limiting))
	for i, l := range a.Limiting {
		names[i] = l.Product.Name
	}
	return names
}

func TestMenuAndStopList(t *testing.T) {
	s := apitest.New(t)
	cream, bacon, eggs := create(t, s, "Сливки 33%"), create(t, s, "Бекон"), create(t, s, "Яйцо")
	dishWith(t, s, "Паста", per(cream, 100), per(bacon, 50))
	dishWith(t, s, "Омлет", per(eggs, 3), per(cream, 50))
	createDish(t, s, "Черновик")
	supply(t, s, cream, 350, oct(20))
	supply(t, s, bacon, 500, oct(20))

	m := menu(t, s)
	require.Len(t, m, 3)

	pasta := m["Паста"]
	require.True(t, pasta.Available)
	require.EqualValues(t, 3, pasta.Portions)
	require.False(t, pasta.Reason.Set)
	require.Equal(t, []oas.LimitingIngredient{{Product: *cream, RequiredQuantity: 100, AvailableQuantity: 350}}, pasta.Limiting,
		"cream limits the portions, with the whole product for display")
	require.Equal(t, []oas.LimitingIngredient{
		{Product: *bacon, RequiredQuantity: 50, AvailableQuantity: 500},
		{Product: *cream, RequiredQuantity: 100, AvailableQuantity: 350},
	}, pasta.Ingredients, "the whole recipe, not only the limiting product")

	omelette := m["Омлет"]
	require.False(t, omelette.Available)
	require.Zero(t, omelette.Portions)
	require.Equal(t, oas.DishAvailabilityReasonInsufficientStock, omelette.Reason.Value)
	require.Equal(t, []oas.LimitingIngredient{{Product: *eggs, RequiredQuantity: 3}}, omelette.Limiting, "eggs were never delivered")

	draft := m["Черновик"]
	require.False(t, draft.Available)
	require.Equal(t, oas.DishAvailabilityReasonNoRecipe, draft.Reason.Value)
	require.Empty(t, draft.Limiting)
	require.Empty(t, draft.Ingredients)

	require.Equal(t, []string{"Омлет", "Черновик"}, stopped(t, s), "by dish name")

	t.Run("two products are missing", func(t *testing.T) {
		dishWith(t, s, "Яичница с беконом", per(eggs, 2), per(bacon, 600))
		require.ElementsMatch(t, []string{"Яйцо", "Бекон"}, limitingProducts(menu(t, s)["Яичница с беконом"]))
	})
	t.Run("pagination", func(t *testing.T) {
		page, err := s.Client.GetMenu(ctx, oas.GetMenuParams{PointId: s.PointID, Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(1)})
		require.NoError(t, err)
		require.EqualValues(t, 4, page.Total)
		require.Equal(t, []string{"Паста", "Черновик"}, []string{page.Items[0].Dish.Name, page.Items[1].Dish.Name})

		stop, err := s.Client.GetStopList(ctx, oas.GetStopListParams{PointId: s.PointID, Limit: oas.NewOptInt32(2)})
		require.NoError(t, err)
		require.EqualValues(t, 3, stop.Total)
		require.Len(t, stop.Items, 2)

		beyond, err := s.Client.GetStopList(ctx, oas.GetStopListParams{PointId: s.PointID, Offset: oas.NewOptInt32(10)})
		require.NoError(t, err)
		require.Empty(t, beyond.Items)
		require.EqualValues(t, 3, beyond.Total)
	})
}

func TestEmptyStopList(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dishWith(t, s, "Соус", per(cream, 10))
	supply(t, s, cream, 100, oct(20))

	require.Empty(t, stopped(t, s))
	page, err := s.Client.GetStopList(ctx, oas.GetStopListParams{PointId: s.PointID})
	require.NoError(t, err)
	require.NotNil(t, page.Items, "an empty list, not null")
}

// Блюдо входит в стоп-лист и выходит из него без действий пользователя.
func TestStopListFollowsTheStock(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	pasta := dishWith(t, s, "Паста", per(cream, 100), per(bacon, 50))
	require.Equal(t, []string{"Паста"}, stopped(t, s), "nothing was delivered yet")

	creamBatch := supply(t, s, cream, 200, oct(15))
	baconBatch := supply(t, s, bacon, 100, oct(20))
	require.Empty(t, stopped(t, s))
	require.EqualValues(t, 2, menu(t, s)["Паста"].Portions)

	t.Run("order", func(t *testing.T) {
		placed, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 2)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		require.Equal(t, oas.OrderStatusAccepted, placed.Status)
		require.Equal(t, []string{"Паста"}, stopped(t, s), "right after the order, without any projection")
		require.ElementsMatch(t, []string{"Сливки 33%", "Бекон"}, limitingProducts(menu(t, s)["Паста"]))
	})
	t.Run("a registered delivery is not stock", func(t *testing.T) {
		registered := registerDelivery(t, s, deliveryItem(cream, 300, oct(15)), deliveryItem(bacon, 300, oct(20)))
		require.Equal(t, []string{"Паста"}, stopped(t, s))

		_, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
		require.NoError(t, err)
		require.Empty(t, stopped(t, s), "the dish leaves the stop list after the approval")
		require.EqualValues(t, 3, menu(t, s)["Паста"].Portions)
	})
	t.Run("write-off", func(t *testing.T) {
		batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: bacon.ID})
		require.NoError(t, err)
		for _, batch := range batches.Items {
			if batch.Quantity == 0 {
				continue
			}
			_, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{
				Quantity: batch.Quantity - 20, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "c",
			}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batch.ID})
			require.NoError(t, err)
		}
		require.Equal(t, []string{"Паста"}, stopped(t, s))
		limiting := menu(t, s)["Паста"].Limiting
		require.Equal(t, []string{"Бекон"}, limitingProducts(menu(t, s)["Паста"]))
		require.EqualValues(t, 20, limiting[0].AvailableQuantity)
		require.EqualValues(t, 50, limiting[0].RequiredQuantity)
	})
	t.Run("recipe change", func(t *testing.T) {
		_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 100)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
		require.NoError(t, err)
		require.Empty(t, stopped(t, s), "the missing ingredient is no longer in the recipe")
	})
	t.Run("expiry without any event", func(t *testing.T) {
		before := countEvents(t, s)
		s.Clock.Set(oct(15)) // срок всех партий сливок истёк
		require.Equal(t, []string{"Паста"}, stopped(t, s))
		require.Equal(t, before, countEvents(t, s), "nothing was written")

		limiting := menu(t, s)["Паста"].Limiting
		require.Len(t, limiting, 1)
		require.Zero(t, limiting[0].AvailableQuantity)
		require.EqualValues(t, 300, limiting[0].ExpiredQuantity, "the product is there, but expired")
	})
	_, _ = creamBatch, baconBatch
}

// Меню отвечает на тот же вопрос, что оформление заказа, и даёт тот же ответ:
// N порций - заказ на N принимается, на N+1 отклоняется.
func TestMenuAgreesWithOrders(t *testing.T) {
	for _, tt := range []struct {
		creamStock, baconStock int64
		portions               int64
	}{
		{350, 500, 3}, {300, 500, 3}, {399, 50, 1}, {1000, 49, 0}, {99, 1000, 0}, {100, 50, 1}, {0, 0, 0},
	} {
		t.Run(fmt.Sprintf("cream %d, bacon %d", tt.creamStock, tt.baconStock), func(t *testing.T) {
			s := apitest.New(t)
			cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
			pasta := dishWith(t, s, "Паста", per(cream, 100), per(bacon, 50))
			if tt.creamStock > 0 {
				supply(t, s, cream, tt.creamStock, oct(20))
			}
			if tt.baconStock > 0 {
				supply(t, s, bacon, tt.baconStock, oct(20))
			}

			available := menu(t, s)["Паста"]
			require.Equal(t, tt.portions, available.Portions)
			require.Equal(t, tt.portions > 0, available.Available)

			// на одну порцию больше - отказ; остатки при этом не меняются
			rejected, err := s.Client.PlaceOrder(ctx, order(portions(pasta, available.Portions+1)), oas.PlaceOrderParams{PointId: s.PointID})
			require.NoError(t, err)
			require.Equal(t, oas.OrderStatusRejected, rejected.Status)
			if !available.Available {
				// каждый недостающий продукт заказа одной порции назван в стоп-листе
				require.Equal(t, []string{"Паста"}, stopped(t, s))
				short := map[string]bool{}
				for _, l := range available.Limiting {
					short[l.Product.ID.String()] = true
				}
				for _, shortage := range rejected.Shortages {
					require.True(t, short[shortage.ProductID.String()], "shortage of the order is listed in the stop list")
				}
				require.Len(t, rejected.Shortages, len(available.Limiting))
				return
			}

			// ровно столько, сколько показано, - принят
			accepted, err := s.Client.PlaceOrder(ctx, order(portions(pasta, available.Portions)), oas.PlaceOrderParams{PointId: s.PointID})
			require.NoError(t, err)
			require.Equal(t, oas.OrderStatusAccepted, accepted.Status)
			require.Equal(t, []string{"Паста"}, stopped(t, s), "everything the menu promised is consumed")
		})
	}
}
