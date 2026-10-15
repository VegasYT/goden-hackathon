//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
)

func createDish(t *testing.T, s apitest.Server, name string) *oas.Dish {
	t.Helper()
	dish, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: name}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	return dish
}

func recipe(ingredients ...oas.Ingredient) *oas.Recipe {
	return &oas.Recipe{Ingredients: ingredients}
}

// used - ингредиент в ответе: с названием и единицей продукта.
func used(product *oas.Product, quantity int64) oas.DishIngredient {
	return oas.DishIngredient{ProductID: product.ID, Quantity: quantity, ProductName: product.Name, Unit: product.Unit}
}

// recipeOf - рецептура блюда в виде запроса: продукты и количества.
func recipeOf(dish *oas.Dish) []oas.Ingredient {
	ingredients := make([]oas.Ingredient, len(dish.Ingredients))
	for i, ingredient := range dish.Ingredients {
		ingredients[i] = oas.Ingredient{ProductID: ingredient.ProductID, Quantity: ingredient.Quantity}
	}
	return ingredients
}

func per(product *oas.Product, quantity int64) oas.Ingredient {
	return oas.Ingredient{ProductID: product.ID, Quantity: quantity}
}

func TestCreateAndGetDish(t *testing.T) {
	s := apitest.New(t)

	created, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: "  Паста карбонара "}, oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)

	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, "Паста карбонара", created.Name)
	require.Empty(t, created.Ingredients)

	read, err := s.Client.GetDish(ctx, oas.GetDishParams{CompanyId: s.CompanyID, DishId: created.ID})
	require.NoError(t, err)
	require.Equal(t, created.ID, read.ID)
	require.Empty(t, read.Ingredients)

	// статус создания и пустой список, а не null, в теле ответа
	resp, body := raw(t, s, http.MethodPost, companyPath(s)+"/dishes", `{"name":"Суп"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
	require.Contains(t, string(body), `"ingredients":[]`)
}

func TestChangeRecipeThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	dish := createDish(t, s, "Паста карбонара")

	changed, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80), per(bacon, 60)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)

	// ингредиенты возвращаются по названию продукта
	require.Equal(t, []oas.DishIngredient{used(bacon, 60), used(cream, 80)}, changed.Ingredients)
	read, err := s.Client.GetDish(ctx, oas.GetDishParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	require.Equal(t, changed, read)

	// новая рецептура заменяет прежнюю целиком
	replaced, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 100)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	require.Equal(t, []oas.DishIngredient{used(cream, 100)}, replaced.Ingredients)
}

func TestListDishes(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	createDish(t, s, "Суп")
	pasta := createDish(t, s, "Паста")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	require.NoError(t, err)

	page, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Len(t, page.Items, 2)
	require.Equal(t, "Паста", page.Items[0].Name)
	require.Equal(t, []oas.DishIngredient{used(cream, 80)}, page.Items[0].Ingredients)
	require.Equal(t, "Суп", page.Items[1].Name)
	require.Empty(t, page.Items[1].Ingredients)

	second, err := s.Client.ListDishes(ctx, oas.ListDishesParams{CompanyId: s.CompanyID, Limit: oas.NewOptInt32(1), Offset: oas.NewOptInt32(1)})
	require.NoError(t, err)
	require.Equal(t, int64(2), second.Total)
	require.Len(t, second.Items, 1)
	require.Equal(t, "Суп", second.Items[0].Name)
}

func TestRecipeRejected(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dish := createDish(t, s, "Паста карбонара")
	original := recipe(per(cream, 80))
	_, err := s.Client.ChangeRecipe(ctx, original, oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	unknown := uuid.Must(uuid.NewV7())

	t.Run("unknown product", func(t *testing.T) {
		_, err := s.Client.ChangeRecipe(ctx,
			recipe(per(cream, 80), oas.Ingredient{ProductID: unknown, Quantity: 10}), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})

		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "unknown-reference", problem.Type)
		require.Contains(t, problem.Detail.Value, unknown.String())
	})

	t.Run("product listed twice", func(t *testing.T) {
		_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80), per(cream, 20)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})

		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "validation-error", problem.Type)
		require.Len(t, problem.Errors, 1)
		require.Equal(t, "ingredients", problem.Errors[0].Field)
	})

	path := companyPath(s) + "/dishes/" + dish.ID.String() + "/recipe"
	notByContract := map[string]string{
		"empty list":       `{"ingredients":[]}`,
		"zero quantity":    `{"ingredients":[{"product_id":"` + cream.ID.String() + `","quantity":0}]}`,
		"no ingredients":   `{}`,
		"quantity as text": `{"ingredients":[{"product_id":"` + cream.ID.String() + `","quantity":"много"}]}`,
	}
	for name, body := range notByContract {
		t.Run(name, func(t *testing.T) {
			resp, data := raw(t, s, http.MethodPut, path, body, nil)
			requireRawProblem(t, resp, data, http.StatusBadRequest)
		})
	}

	t.Run("unknown dish", func(t *testing.T) {
		_, err := s.Client.ChangeRecipe(ctx, original, oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: unknown})
		requireProblem(t, err, http.StatusNotFound)

		_, err = s.Client.GetDish(ctx, oas.GetDishParams{CompanyId: s.CompanyID, DishId: unknown})
		requireProblem(t, err, http.StatusNotFound)
	})

	// ни один отклонённый запрос не изменил рецептуру
	read, err := s.Client.GetDish(ctx, oas.GetDishParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	require.Equal(t, original.Ingredients, recipeOf(read))
}

func TestDishNameConflictAndValidation(t *testing.T) {
	s := apitest.New(t)
	createDish(t, s, "Паста карбонара")

	_, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: "ПАСТА КАРБОНАРА"}, oas.CreateDishParams{CompanyId: s.CompanyID})
	problem := requireProblem(t, err, http.StatusConflict)
	require.Equal(t, "conflict", problem.Type)

	_, err = s.Client.CreateDish(ctx, &oas.NewDish{Name: "   "}, oas.CreateDishParams{CompanyId: s.CompanyID})
	problem = requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, []oas.FieldError{{Field: "name", Message: "не должно быть пустым"}}, problem.Errors)
}

func TestChangeRecipeIdempotencyKey(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dish := createDish(t, s, "Паста карбонара")
	params := oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID, IdempotencyKey: oas.NewOptString("recipe-1")}

	first, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80)), params)
	require.NoError(t, err)
	second, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80)), params)
	require.NoError(t, err)

	require.Equal(t, first, second)
	events, err := eventstore.ReadStream(ctx, s.Pool, catalog.DishStream(dish.ID), 1)
	require.NoError(t, err)
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	require.Equal(t, []string{"DishCreated", "RecipeChanged"}, types, "the repeated request wrote no second event")
}

// Все операции контракта реализованы: ни одна не отвечает заглушкой 501.
// operation - запрос, который доходит до обработчика операции контракта.
type operation struct {
	name, method, path, body string
}

// operations возвращает по одному запросу на каждую операцию контракта; id подставляется
// во все идентификаторы пути и тела. Новая операция добавляется сюда: это проверяет
// TestEveryOperationHasARequest.
func operations(id string) []operation {
	return []operation{
		{oas.CreateProductOperation, http.MethodPost, "/companies/" + id + "/products", `{"name":"A","unit":"g"}`},
		{oas.ListProductsOperation, http.MethodGet, "/companies/" + id + "/products", ""},
		{oas.GetProductOperation, http.MethodGet, "/companies/" + id + "/products/" + id, ""},
		{oas.ChangeProductOperation, http.MethodPatch, "/companies/" + id + "/products/" + id, `{"name":"A"}`},
		{oas.ArchiveProductOperation, http.MethodPost, "/companies/" + id + "/products/" + id + "/archive", ""},
		{oas.RestoreProductOperation, http.MethodPost, "/companies/" + id + "/products/" + id + "/restore", ""},
		{oas.ListProductCategoriesOperation, http.MethodGet, "/companies/" + id + "/product-categories", ""},
		{oas.CreateDishOperation, http.MethodPost, "/companies/" + id + "/dishes", `{"name":"B"}`},
		{oas.ListDishesOperation, http.MethodGet, "/companies/" + id + "/dishes", ""},
		{oas.GetDishOperation, http.MethodGet, "/companies/" + id + "/dishes/" + id, ""},
		{oas.RenameDishOperation, http.MethodPatch, "/companies/" + id + "/dishes/" + id, `{"name":"B"}`},
		{oas.ArchiveDishOperation, http.MethodPost, "/companies/" + id + "/dishes/" + id + "/archive", ""},
		{oas.RestoreDishOperation, http.MethodPost, "/companies/" + id + "/dishes/" + id + "/restore", ""},
		{oas.ChangeRecipeOperation, http.MethodPut, "/companies/" + id + "/dishes/" + id + "/recipe", `{"ingredients":[{"product_id":"` + id + `","quantity":1}]}`},
		{oas.RegisterDeliveryOperation, http.MethodPost, "/points/" + id + "/deliveries", `{"number":"N","delivered_at":"2026-10-10T09:00:00Z","items":[{"product_id":"` + id + `","declared_quantity":1,"expires_at":"2026-10-15T00:00:00Z"}]}`},
		{oas.ListDeliveriesOperation, http.MethodGet, "/points/" + id + "/deliveries", ""},
		{oas.GetDeliveryHistoryOperation, http.MethodGet, "/points/" + id + "/deliveries/" + id + "/history", ""},
		{oas.ListProductMovementsOperation, http.MethodGet, "/points/" + id + "/products/" + id + "/movements", ""},
		{oas.GetStockOperation, http.MethodGet, "/points/" + id + "/stock", ""},
		{oas.GetStockSummaryOperation, http.MethodGet, "/points/" + id + "/stock/summary", ""},
		{oas.ListExpiringBatchesOperation, http.MethodGet, "/points/" + id + "/batches/expiring", ""},
		{oas.GetMenuOperation, http.MethodGet, "/points/" + id + "/menu", ""},
		{oas.GetStopListOperation, http.MethodGet, "/points/" + id + "/stop-list", ""},
		{oas.ListDishRecommendationsOperation, http.MethodGet, "/points/" + id + "/recommendations/dishes", ""},
		{oas.ListHotOfferSettingsOperation, http.MethodGet, "/points/" + id + "/recommendations/dishes/settings", ""},
		{oas.SetHotOfferOperation, http.MethodPut, "/points/" + id + "/recommendations/dishes/" + id, `{"mode":"pinned"}`},
		{oas.ListDeliveryRecommendationsOperation, http.MethodGet, "/points/" + id + "/recommendations/deliveries", ""},
		{oas.GetWriteOffStatisticsOperation, http.MethodGet, "/points/" + id + "/analytics/write-offs?from=2026-10-10T00:00:00Z&to=2026-10-11T00:00:00Z", ""},
		{oas.GetWriteOffDaysOperation, http.MethodGet, "/points/" + id + "/analytics/write-offs/days", ""},
		{oas.GetShortageStatisticsOperation, http.MethodGet, "/points/" + id + "/analytics/shortages?from=2026-10-10T00:00:00Z&to=2026-10-11T00:00:00Z", ""},
		{oas.GetStopListForecastOperation, http.MethodGet, "/points/" + id + "/analytics/stop-list-forecast", ""},
		{oas.ListOrdersOperation, http.MethodGet, "/points/" + id + "/orders", ""},
		{oas.GetDeliveryOperation, http.MethodGet, "/points/" + id + "/deliveries/" + id, ""},
		{oas.ApproveDeliveryOperation, http.MethodPost, "/points/" + id + "/deliveries/" + id + "/approval", `{"items":[{"item_id":"` + id + `","accepted_quantity":1,"rejected_quantity":0,"quarantined_quantity":0}]}`},
		{oas.ResolveQuarantineOperation, http.MethodPost, "/points/" + id + "/deliveries/" + id + "/items/" + id + "/quarantine-resolution", `{"accepted_quantity":1,"rejected_quantity":0}`},
		{oas.CorrectDeliveryApprovalOperation, http.MethodPost, "/points/" + id + "/deliveries/" + id + "/corrections", `{"reason":"r","items":[{"item_id":"` + id + `","accepted_quantity":1,"rejected_quantity":0,"quarantined_quantity":0}]}`},
		{oas.ListProductBatchesOperation, http.MethodGet, "/points/" + id + "/products/" + id + "/batches", ""},
		{oas.GetBatchOperation, http.MethodGet, "/points/" + id + "/batches/" + id, ""},
		{oas.WriteOffStockOperation, http.MethodPost, "/points/" + id + "/batches/" + id + "/write-offs", `{"quantity":1,"reason":"other","comment":"c"}`},
		{oas.AdjustStockOperation, http.MethodPost, "/points/" + id + "/batches/" + id + "/adjustments", `{"delta":1,"reason":"r"}`},
		{oas.PlaceOrderOperation, http.MethodPost, "/points/" + id + "/orders", `{"items":[{"dish_id":"` + id + `","portions":1}]}`},
		{oas.GetOrderOperation, http.MethodGet, "/points/" + id + "/orders/" + id, ""},
		{oas.CancelOrderOperation, http.MethodPost, "/points/" + id + "/orders/" + id + "/cancel", ""},
		{oas.GetDeliveriesSummaryOperation, http.MethodGet, "/points/" + id + "/deliveries/summary", ""},
		{oas.GetGeneratorSettingsOperation, http.MethodGet, "/points/" + id + "/generator", ""},
		{oas.ListGeneratorPointsOperation, http.MethodGet, "/generator/points", ""},
		{oas.UpdateGeneratorSettingsOperation, http.MethodPut, "/points/" + id + "/generator", `{"running":false,"deliveries_per_minute":1,"orders_per_minute":1}`},
		{oas.GetMigomOnPointOperation, http.MethodGet, "/points/" + id + "/migom", ""},
		{oas.SetMigomOnPointOperation, http.MethodPut, "/points/" + id + "/migom", `{"outlet_id":null}`},
		{oas.ListMigomPointsOperation, http.MethodGet, "/migom/points", ""},
		{oas.ListSuppliersOperation, http.MethodGet, "/companies/" + id + "/suppliers", ""},
		{oas.CreateSupplierOperation, http.MethodPost, "/companies/" + id + "/suppliers", `{"name":"S","point_ids":["` + id + `"]}`},
		{oas.UpdateSupplierOperation, http.MethodPut, "/companies/" + id + "/suppliers/" + id, `{"name":"S","point_ids":["` + id + `"]}`},
		{oas.IssueSupplierKeyOperation, http.MethodPost, "/companies/" + id + "/suppliers/" + id + "/key", ""},
		{oas.RevokeSupplierKeyOperation, http.MethodDelete, "/companies/" + id + "/suppliers/" + id + "/key", ""},
		{oas.GetSupplierSelfOperation, http.MethodGet, "/supplier", ""},
		{oas.ListSupplierProductsOperation, http.MethodGet, "/supplier/products", ""},
		{oas.StreamEventsOperation, http.MethodGet, "/points/" + id + "/events", ""},
		{oas.GetMeOperation, http.MethodGet, "/me", ""},
		{oas.CreateCompanyOperation, http.MethodPost, "/companies", `{"name":"C"}`},
		{oas.GetCompanyOperation, http.MethodGet, "/companies/" + id, ""},
		{oas.RenameCompanyOperation, http.MethodPatch, "/companies/" + id, `{"name":"C2"}`},
		{oas.TransferCompanyOperation, http.MethodPut, "/companies/" + id + "/owner", `{"email":"a@goden.local"}`},
		{oas.DisableCompanyOperation, http.MethodPost, "/companies/" + id + "/disable", ""},
		{oas.ListPointsOperation, http.MethodGet, "/companies/" + id + "/points", ""},
		{oas.CreatePointOperation, http.MethodPost, "/companies/" + id + "/points", `{"name":"P"}`},
		{oas.GetPointOperation, http.MethodGet, "/points/" + id, ""},
		{oas.RenamePointOperation, http.MethodPatch, "/points/" + id, `{"name":"P2"}`},
		{oas.DisablePointOperation, http.MethodPost, "/points/" + id + "/disable", ""},
		{oas.ListStaffOperation, http.MethodGet, "/points/" + id + "/staff", ""},
		{oas.AssignStaffOperation, http.MethodPost, "/points/" + id + "/staff", `{"email":"a@goden.local","role":"cook"}`},
		{oas.ChangeStaffRoleOperation, http.MethodPut, "/points/" + id + "/staff/" + id, `{"role":"cook"}`},
		{oas.ListPointDishesOperation, http.MethodGet, "/points/" + id + "/dishes", ""},
		{oas.SetPointTelegramChatOperation, http.MethodPut, "/points/" + id + "/telegram-chat", `{"chat_id":-100123}`},
		{oas.SetDishOnPointOperation, http.MethodPut, "/points/" + id + "/dishes/" + id, `{"enabled":false}`},
		{oas.RemoveStaffOperation, http.MethodDelete, "/points/" + id + "/staff/" + id, ""},
	}
}

func TestNoOperationIsUnimplemented(t *testing.T) {
	s := apitest.New(t)
	for _, op := range operations(uuid.Must(uuid.NewV7()).String()) {
		var resp *http.Response
		var body []byte
		if op.name == oas.StreamEventsOperation {
			resp, body = rawSSE(t, s, nil)
		} else {
			resp, body = raw(t, s, op.method, api.Prefix+op.path, op.body, nil)
		}
		require.NotEqual(t, http.StatusNotImplemented, resp.StatusCode, "%s: %s", op.name, body)
	}
}
