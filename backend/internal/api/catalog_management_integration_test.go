//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
)

func productIn(t *testing.T, s apitest.Server, name, category string, unit oas.Unit) *oas.Product {
	t.Helper()
	product, err := s.Client.CreateProduct(ctx, &oas.NewProduct{Name: name, Unit: unit, Category: oas.NewOptString(category)},
		oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	return product
}

func listProducts(t *testing.T, s apitest.Server, params oas.ListProductsParams) ([]string, int64) {
	t.Helper()
	params.CompanyId = s.CompanyID
	page, err := s.Client.ListProducts(ctx, params)
	require.NoError(t, err)
	names := make([]string, len(page.Items))
	for i, product := range page.Items {
		names[i] = product.Name
	}
	return names, page.Total
}

func listDishes(t *testing.T, s apitest.Server, params oas.ListDishesParams) []string {
	t.Helper()
	params.CompanyId = s.CompanyID
	page, err := s.Client.ListDishes(ctx, params)
	require.NoError(t, err)
	names := make([]string, len(page.Items))
	for i, dish := range page.Items {
		names[i] = dish.Name
	}
	return names
}

// streamEvents - типы событий потока по порядку версий.
func streamEvents(t *testing.T, s apitest.Server, stream string) []string {
	t.Helper()
	rows, err := s.Pool.Query(ctx, `SELECT event_type FROM events WHERE stream_id = $1 ORDER BY stream_version`, stream)
	require.NoError(t, err)
	defer rows.Close()
	var types []string
	for rows.Next() {
		var eventType string
		require.NoError(t, rows.Scan(&eventType))
		types = append(types, eventType)
	}
	return types
}

func requireFieldError(t *testing.T, problem oas.Problem, field, code string) {
	t.Helper()
	require.Len(t, problem.Errors, 1)
	require.Equal(t, field, problem.Errors[0].Field)
	require.Equal(t, code, problem.Errors[0].Code.Value)
}

func TestProductListSearchFiltersAndCategories(t *testing.T) {
	s := apitest.New(t)
	productIn(t, s, "Куриное филе", "Мясо", oas.UnitG)
	productIn(t, s, "Филе лосося", "Рыба", oas.UnitG)
	bacon := productIn(t, s, "Бекон", "Мясо", oas.UnitG)
	productIn(t, s, "Молоко", "Молочные продукты", oas.UnitMl)
	productIn(t, s, "Сливки 33%", "Молочные продукты", oas.UnitMl)
	productIn(t, s, "Вода", "", oas.UnitMl)

	names, total := listProducts(t, s, oas.ListProductsParams{Q: oas.NewOptString("ФИЛЕ")})
	require.Equal(t, []string{"Куриное филе", "Филе лосося"}, names)
	require.EqualValues(t, 2, total)

	names, total = listProducts(t, s, oas.ListProductsParams{Q: oas.NewOptString("_")})
	require.Empty(t, names, "a pattern sign in the query is an ordinary character")
	require.Zero(t, total)
	names, _ = listProducts(t, s, oas.ListProductsParams{Q: oas.NewOptString("3%")})
	require.Equal(t, []string{"Сливки 33%"}, names)

	names, _ = listProducts(t, s, oas.ListProductsParams{Category: oas.NewOptString("Молочные продукты"), Q: oas.NewOptString("сли")})
	require.Equal(t, []string{"Сливки 33%"}, names)

	names, _ = listProducts(t, s, oas.ListProductsParams{Sort: oas.NewOptListProductsSort(oas.ListProductsSortCategory)})
	require.Equal(t, []string{"Молоко", "Сливки 33%", "Бекон", "Куриное филе", "Филе лосося", "Вода"}, names,
		"by category, by name inside it, products without a category last")

	names, total = listProducts(t, s, oas.ListProductsParams{Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(2)})
	require.Equal(t, []string{"Куриное филе", "Молоко"}, names)
	require.EqualValues(t, 6, total)

	_, err := s.Client.ArchiveProduct(ctx, oas.ArchiveProductParams{CompanyId: s.CompanyID, ProductId: bacon.ID})
	require.NoError(t, err)
	names, _ = listProducts(t, s, oas.ListProductsParams{})
	require.NotContains(t, names, "Бекон", "the list without a status shows only working products")
	names, _ = listProducts(t, s, oas.ListProductsParams{Status: oas.NewOptCatalogStatus(oas.CatalogStatusArchived)})
	require.Equal(t, []string{"Бекон"}, names)

	categories, err := s.Client.ListProductCategories(ctx, oas.ListProductCategoriesParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, []oas.ProductCategory{
		{Name: "Молочные продукты", Products: 2},
		{Name: "Мясо", Products: 1},
		{Name: "Рыба", Products: 1},
	}, categories.Items, "an archived product is not counted, an empty category is not listed")
}

func TestProductChangesAndUnitLock(t *testing.T) {
	s := apitest.New(t)
	shelfLife := oas.NewOptInt32(5)
	cream, err := s.Client.CreateProduct(ctx, &oas.NewProduct{
		Name: "Сливки 3%", Unit: oas.UnitMl, Category: oas.NewOptString("Молочные продукты"), ShelfLifeDays: shelfLife,
	}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.False(t, cream.Archived)
	require.Equal(t, oas.NewOptInt64(0), cream.Dishes)
	require.Equal(t, oas.NewOptBool(false), cream.UnitLocked)

	t.Run("a new product changes its unit", func(t *testing.T) {
		eggs := productIn(t, s, "Яйцо куриное", "Яйца", oas.UnitG)
		changed, err := s.Client.ChangeProduct(ctx, &oas.ProductChange{Unit: oas.NewOptUnit(oas.UnitPcs)},
			oas.ChangeProductParams{CompanyId: s.CompanyID, ProductId: eggs.ID})
		require.NoError(t, err)
		require.Equal(t, oas.UnitPcs, changed.Unit)
		require.Equal(t, []string{"ProductCreated", "ProductChanged"}, streamEvents(t, s, catalog.ProductStream(eggs.ID)))
	})

	t.Run("rename shows everywhere and clearing the shelf life", func(t *testing.T) {
		delivery := registerDelivery(t, s, deliveryItem(cream, 1000, oct(20)))
		cleared := oas.OptNilInt32{}
		cleared.SetToNull()
		changed, err := s.Client.ChangeProduct(ctx, &oas.ProductChange{
			Name: oas.NewOptString("  Сливки 33% "), ShelfLifeDays: cleared,
		}, oas.ChangeProductParams{CompanyId: s.CompanyID, ProductId: cream.ID})
		require.NoError(t, err)
		require.Equal(t, "Сливки 33%", changed.Name)
		require.False(t, changed.ShelfLifeDays.Set, "the shelf life is cleared")
		require.Equal(t, oas.UnitMl, changed.Unit, "fields not sent stay as they were")

		read, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: delivery.ID})
		require.NoError(t, err)
		require.Equal(t, "Сливки 33%", read.Items[0].ProductName.Value, "an earlier delivery shows the new name")
	})

	t.Run("a product of an unapproved delivery keeps its unit", func(t *testing.T) {
		_, err := s.Client.ChangeProduct(ctx, &oas.ProductChange{Unit: oas.NewOptUnit(oas.UnitG)},
			oas.ChangeProductParams{CompanyId: s.CompanyID, ProductId: cream.ID})
		problem := requireProblem(t, err, http.StatusConflict)
		require.Equal(t, "unit-locked", problem.Type)
	})

	t.Run("a product of a recipe keeps its unit, its dishes are listed", func(t *testing.T) {
		milk := productIn(t, s, "Молоко", "Молочные продукты", oas.UnitMl)
		dishWith(t, s, "Омлет", per(milk, 50))
		dishWith(t, s, "Каша", per(milk, 200))

		read, err := s.Client.GetProduct(ctx, oas.GetProductParams{CompanyId: s.CompanyID, ProductId: milk.ID})
		require.NoError(t, err)
		require.Equal(t, oas.NewOptInt64(2), read.Dishes)
		require.Equal(t, oas.NewOptBool(true), read.UnitLocked)
		require.Len(t, read.UsedIn, 2)
		require.Equal(t, "Каша", read.UsedIn[0].Name)
		require.Equal(t, "Омлет", read.UsedIn[1].Name)

		_, err = s.Client.ChangeProduct(ctx, &oas.ProductChange{Unit: oas.NewOptUnit(oas.UnitG)},
			oas.ChangeProductParams{CompanyId: s.CompanyID, ProductId: milk.ID})
		requireProblem(t, err, http.StatusConflict)
	})

	t.Run("a taken name points at the field", func(t *testing.T) {
		bacon := productIn(t, s, "Бекон", "Мясо", oas.UnitG)
		_, err := s.Client.ChangeProduct(ctx, &oas.ProductChange{Name: oas.NewOptString("сливки 33%")},
			oas.ChangeProductParams{CompanyId: s.CompanyID, ProductId: bacon.ID})
		problem := requireProblem(t, err, http.StatusConflict)
		require.Equal(t, "conflict", problem.Type)
		requireFieldError(t, problem, "name", "taken")
	})

	t.Run("the same values write no event", func(t *testing.T) {
		before := streamEvents(t, s, catalog.ProductStream(cream.ID))
		_, err := s.Client.ChangeProduct(ctx, &oas.ProductChange{Name: oas.NewOptString("Сливки 33%"), Unit: oas.NewOptUnit(oas.UnitMl)},
			oas.ChangeProductParams{CompanyId: s.CompanyID, ProductId: cream.ID})
		require.NoError(t, err)
		require.Equal(t, before, streamEvents(t, s, catalog.ProductStream(cream.ID)))
	})
}

func TestProductArchive(t *testing.T) {
	s := apitest.New(t)
	bacon := productIn(t, s, "Бекон", "Мясо", oas.UnitG)
	dishWith(t, s, "Паста карбонара", per(bacon, 60))
	tomatoes := productIn(t, s, "Томаты", "Овощи", oas.UnitG)
	batchID := supply(t, s, tomatoes, 1000, oct(20))
	basil := productIn(t, s, "Базилик", "Зелень", oas.UnitG)

	_, err := s.Client.ArchiveProduct(ctx, oas.ArchiveProductParams{CompanyId: s.CompanyID, ProductId: bacon.ID})
	problem := requireProblem(t, err, http.StatusConflict)
	require.Equal(t, "product-in-use", problem.Type)
	require.Contains(t, problem.Detail.Value, "Паста карбонара")

	for _, product := range []*oas.Product{tomatoes, basil} {
		archived, err := s.Client.ArchiveProduct(ctx, oas.ArchiveProductParams{CompanyId: s.CompanyID, ProductId: product.ID})
		require.NoError(t, err)
		require.True(t, archived.Archived)
	}

	stock, err := s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID})
	require.NoError(t, err)
	var onStock []string
	for _, level := range stock.Items {
		onStock = append(onStock, level.Product.Name)
	}
	require.Contains(t, onStock, "Томаты", "an archived product stays on the stock while it has batches")
	require.NotContains(t, onStock, "Базилик", "without batches an archived product is not on the stock")

	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{Quantity: 100, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "помяты"},
		oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
	require.NoError(t, err, "a batch of an archived product is still written off")

	_, err = s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-2", DeliveredAt: s.Clock.Now(), Items: []oas.NewDeliveryItem{deliveryItem(tomatoes, 500, oct(20))},
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	problem = requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "archived-reference", problem.Type)

	_, err = s.Client.CreateDish(ctx, &oas.NewDish{Name: "Брускетта", Ingredients: []oas.Ingredient{per(tomatoes, 100)}},
		oas.CreateDishParams{CompanyId: s.CompanyID})
	requireProblem(t, err, http.StatusUnprocessableEntity)

	_, err = s.Client.CreateProduct(ctx, newProduct("томаты"), oas.CreateProductParams{CompanyId: s.CompanyID})
	problem = requireProblem(t, err, http.StatusConflict)
	requireFieldError(t, problem, "name", "taken_by_archived")

	restored, err := s.Client.RestoreProduct(ctx, oas.RestoreProductParams{CompanyId: s.CompanyID, ProductId: tomatoes.ID})
	require.NoError(t, err)
	require.False(t, restored.Archived)
	registerDelivery(t, s, deliveryItem(tomatoes, 500, oct(20)))
	require.Equal(t, []string{"ProductCreated", "ProductArchived", "ProductRestored"},
		streamEvents(t, s, catalog.ProductStream(tomatoes.ID)))
}

func TestDishCreationWithRecipeRenameAndList(t *testing.T) {
	s := apitest.New(t)
	eggs := productIn(t, s, "Яйцо куриное", "Яйца", oas.UnitPcs)
	milk := productIn(t, s, "Молоко", "Молочные продукты", oas.UnitMl)

	omelet, err := s.Client.CreateDish(ctx, &oas.NewDish{Name: "Омлет", Ingredients: []oas.Ingredient{per(eggs, 3), per(milk, 50)}},
		oas.CreateDishParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	require.Equal(t, []oas.DishIngredient{
		{ProductID: milk.ID, Quantity: 50, ProductName: "Молоко", Unit: oas.UnitMl},
		{ProductID: eggs.ID, Quantity: 3, ProductName: "Яйцо куриное", Unit: oas.UnitPcs},
	}, omelet.Ingredients)
	require.Equal(t, []string{"DishCreated", "RecipeChanged"}, streamEvents(t, s, catalog.DishStream(omelet.ID)))

	events := countEvents(t, s)
	_, err = s.Client.CreateDish(ctx, &oas.NewDish{Name: "Яичница", Ingredients: []oas.Ingredient{per(eggs, 2), per(eggs, 1)}},
		oas.CreateDishParams{CompanyId: s.CompanyID})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "ingredients", problem.Errors[0].Field)
	require.Equal(t, events, countEvents(t, s), "a dish with a wrong recipe is not created")

	createDish(t, s, "Суп")
	require.Equal(t, []string{"Суп"}, listDishes(t, s, oas.ListDishesParams{Recipe: oas.NewOptListDishesRecipe(oas.ListDishesRecipeMissing)}))
	require.Equal(t, []string{"Омлет"}, listDishes(t, s, oas.ListDishesParams{Q: oas.NewOptString("ОМ")}))

	renamed, err := s.Client.RenameDish(ctx, &oas.DishChange{Name: "Омлет с молоком"}, oas.RenameDishParams{CompanyId: s.CompanyID, DishId: omelet.ID})
	require.NoError(t, err)
	require.Equal(t, "Омлет с молоком", renamed.Name)
	require.Len(t, renamed.Ingredients, 2, "rename keeps the recipe")

	_, err = s.Client.RenameDish(ctx, &oas.DishChange{Name: "суп"}, oas.RenameDishParams{CompanyId: s.CompanyID, DishId: omelet.ID})
	problem = requireProblem(t, err, http.StatusConflict)
	requireFieldError(t, problem, "name", "taken")
}

func TestDishArchiveOnPoints(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	cream := ownProduct(t, n, "Сливки 33%")
	pasta, err := n.owner.CreateDish(ctx, &oas.NewDish{Name: "Паста", Ingredients: []oas.Ingredient{per(cream, 80)}},
		oas.CreateDishParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	_, err = n.owner.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: false},
		oas.SetDishOnPointParams{PointId: n.a.ID, DishId: pasta.ID})
	require.NoError(t, err)

	archived, err := n.owner.ArchiveDish(ctx, oas.ArchiveDishParams{CompanyId: n.company.ID, DishId: pasta.ID})
	require.NoError(t, err)
	require.True(t, archived.Archived)

	menu, err := n.owner.GetMenu(ctx, oas.GetMenuParams{PointId: n.b.ID})
	require.NoError(t, err)
	require.Empty(t, menu.Items, "an archived dish is in no menu")
	_, err = n.owner.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: n.b.ID})
	requireProblem(t, err, http.StatusUnprocessableEntity)

	_, err = n.owner.RestoreDish(ctx, oas.RestoreDishParams{CompanyId: n.company.ID, DishId: pasta.ID})
	require.NoError(t, err)
	for point, enabled := range map[*oas.Point]bool{n.a: false, n.b: true} {
		dishes, err := n.owner.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: point.ID})
		require.NoError(t, err)
		require.Equal(t, []oas.PointDish{{DishID: pasta.ID, Name: "Паста", Enabled: enabled}}, dishes.Items,
			"switching the dish off on a point survives the archive")
	}

	_, err = n.owner.ArchiveDish(ctx, oas.ArchiveDishParams{CompanyId: n.company.ID, DishId: pasta.ID})
	require.NoError(t, err)
	_, err = n.owner.ArchiveProduct(ctx, oas.ArchiveProductParams{CompanyId: n.company.ID, ProductId: cream.ID})
	require.NoError(t, err, "a product of an archived dish goes to the archive")
	_, err = n.owner.RestoreDish(ctx, oas.RestoreDishParams{CompanyId: n.company.ID, DishId: pasta.ID})
	problem := requireProblem(t, err, http.StatusConflict)
	require.Equal(t, "archived-ingredients", problem.Type)
	require.Contains(t, problem.Detail.Value, "Сливки 33%")
}

func TestCatalogChangesInADisabledCompany(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	cream := ownProduct(t, n, "Сливки 33%")
	_, err := n.owner.DisableCompany(ctx, oas.DisableCompanyParams{CompanyId: n.company.ID})
	require.NoError(t, err)

	_, err = n.owner.ChangeProduct(ctx, &oas.ProductChange{Name: oas.NewOptString("Сливки 10%")},
		oas.ChangeProductParams{CompanyId: n.company.ID, ProductId: cream.ID})
	requireProblem(t, err, http.StatusConflict)
	_, err = n.owner.ArchiveProduct(ctx, oas.ArchiveProductParams{CompanyId: n.company.ID, ProductId: cream.ID})
	requireProblem(t, err, http.StatusConflict)
}

func TestNameAndEmailErrorsPointAtTheField(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)

	_, err := n.owner.CreatePoint(ctx, &oas.NewPoint{Name: "Точка на Ленина"}, oas.CreatePointParams{CompanyId: n.company.ID})
	problem := requireProblem(t, err, http.StatusConflict)
	require.Equal(t, "conflict", problem.Type)
	requireFieldError(t, problem, "name", "taken")

	_, err = n.owner.AssignStaff(ctx, &oas.NewStaff{Email: "nobody@goden.local", Role: oas.RoleCook}, oas.AssignStaffParams{PointId: n.a.ID})
	problem = requireProblem(t, err, http.StatusUnprocessableEntity)
	requireFieldError(t, problem, "email", "unknown")

	_, err = n.owner.TransferCompany(ctx, &oas.NewOwner{Email: "nobody@goden.local"}, oas.TransferCompanyParams{CompanyId: n.company.ID})
	problem = requireProblem(t, err, http.StatusUnprocessableEntity)
	requireFieldError(t, problem, "email", "unknown")
}
