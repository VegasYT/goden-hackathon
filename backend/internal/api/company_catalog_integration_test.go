//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func ownProduct(t *testing.T, n network, name string) *oas.Product {
	t.Helper()
	product, err := n.owner.CreateProduct(ctx, &oas.NewProduct{Name: name, Unit: oas.UnitG}, oas.CreateProductParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	return product
}

func ownDish(t *testing.T, n network, name string) *oas.Dish {
	t.Helper()
	dish, err := n.owner.CreateDish(ctx, &oas.NewDish{Name: name}, oas.CreateDishParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	return dish
}

// Каталог меняет только владелец, читает сотрудник любой точки компании, посторонний не видит.
func TestCatalogAccessFollowsTheCompany(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	cream, pasta := ownProduct(t, n, "Сливки"), ownDish(t, n, "Паста")
	require.Equal(t, n.company.ID, cream.CompanyID)
	require.Equal(t, n.company.ID, pasta.CompanyID)

	manager, _ := hire(t, s, n.owner, n.a, "manager", oas.RoleManager)
	cook, _ := hire(t, s, n.owner, n.b, "cook", oas.RoleCook)
	stranger := s.Login(t, "stranger")
	recipe := &oas.Recipe{Ingredients: []oas.Ingredient{{ProductID: cream.ID, Quantity: 80}}}

	// управляющий читает каталог, но не меняет его
	_, err := manager.CreateProduct(ctx, &oas.NewProduct{Name: "Бекон", Unit: oas.UnitG}, oas.CreateProductParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "products.manage")
	_, err = manager.CreateDish(ctx, &oas.NewDish{Name: "Суп"}, oas.CreateDishParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "dishes.manage")
	_, err = manager.ChangeRecipe(ctx, recipe, oas.ChangeRecipeParams{CompanyId: n.company.ID, DishId: pasta.ID})
	requireForbidden(t, err, "dishes.manage")

	for _, staff := range []*oas.Client{manager, cook} {
		dishes, err := staff.ListDishes(ctx, oas.ListDishesParams{CompanyId: n.company.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, dishes.Total)
		_, err = staff.GetProduct(ctx, oas.GetProductParams{CompanyId: n.company.ID, ProductId: cream.ID})
		require.NoError(t, err)
	}

	_, err = stranger.ListProducts(ctx, oas.ListProductsParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "catalog.view")
	_, err = stranger.GetDish(ctx, oas.GetDishParams{CompanyId: n.company.ID, DishId: pasta.ID})
	requireForbidden(t, err, "catalog.view")

	changed, err := n.owner.ChangeRecipe(ctx, recipe, oas.ChangeRecipeParams{CompanyId: n.company.ID, DishId: pasta.ID})
	require.NoError(t, err)
	require.Len(t, changed.Ingredients, 1)
}

// Владелец своей компании не дотягивается до чужого каталога по идентификатору:
// право у него есть только в своей компании, а объект принадлежит другой.
func TestForeignCatalogIsNotReachableThroughOwnCompany(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	cream, pasta := ownProduct(t, n, "Сливки"), ownDish(t, n, "Паста")

	// у пользователя dev своя компания с каталогом
	salt := create(t, s, "Соль")
	soup := createDish(t, s, "Суп")
	recipe := &oas.Recipe{Ingredients: []oas.Ingredient{{ProductID: salt.ID, Quantity: 5}}}

	_, err := s.Client.GetProduct(ctx, oas.GetProductParams{CompanyId: s.CompanyID, ProductId: cream.ID})
	requireProblem(t, err, http.StatusNotFound)
	_, err = s.Client.GetDish(ctx, oas.GetDishParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	requireProblem(t, err, http.StatusNotFound)
	before := countEvents(t, s)
	_, err = s.Client.ChangeRecipe(ctx, recipe, oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	requireProblem(t, err, http.StatusNotFound)

	// чужой продукт в рецептуре своего блюда
	foreign := &oas.Recipe{Ingredients: []oas.Ingredient{{ProductID: cream.ID, Quantity: 5}}}
	_, err = s.Client.ChangeRecipe(ctx, foreign, oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: soup.ID})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "unknown-reference", problem.Type)
	require.Equal(t, before, countEvents(t, s))

	// одинаковые названия в разных компаниях не конфликтуют, списки не смешиваются
	_, err = s.Client.CreateProduct(ctx, &oas.NewProduct{Name: "Сливки", Unit: oas.UnitMl}, oas.CreateProductParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	products, err := n.owner.ListProducts(ctx, oas.ListProductsParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, products.Total)
}

func TestDishIsSwitchedOffOnAPoint(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	pasta := ownDish(t, n, "Паста")
	ownDish(t, n, "Суп")
	manager, _ := hire(t, s, n.owner, n.a, "manager", oas.RoleManager)
	cook, _ := hire(t, s, n.owner, n.a, "cook", oas.RoleCook)

	enabled := func(client *oas.Client, point *oas.Point) map[string]bool {
		page, err := client.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: point.ID})
		require.NoError(t, err)
		require.EqualValues(t, 2, page.Total)
		out := map[string]bool{}
		for _, dish := range page.Items {
			out[dish.Name] = dish.Enabled
		}
		return out
	}
	require.Equal(t, map[string]bool{"Паста": true, "Суп": true}, enabled(cook, n.a))

	off := &oas.DishAvailabilityOnPoint{Enabled: false}
	changed, err := manager.SetDishOnPoint(ctx, off, oas.SetDishOnPointParams{PointId: n.a.ID, DishId: pasta.ID})
	require.NoError(t, err)
	require.Equal(t, &oas.PointDish{DishID: pasta.ID, Name: "Паста", Enabled: false}, changed)
	require.Equal(t, map[string]bool{"Паста": false, "Суп": true}, enabled(cook, n.a))
	require.Equal(t, map[string]bool{"Паста": true, "Суп": true}, enabled(n.owner, n.b), "the other point still sells the dish")

	// повар читает список, но не меняет; управляющий точки А не распоряжается точкой Б
	_, err = cook.SetDishOnPoint(ctx, off, oas.SetDishOnPointParams{PointId: n.a.ID, DishId: pasta.ID})
	requireForbidden(t, err, "menu.manage")
	_, err = manager.SetDishOnPoint(ctx, off, oas.SetDishOnPointParams{PointId: n.b.ID, DishId: pasta.ID})
	requireForbidden(t, err, "menu.manage")
	_, err = manager.ListPointDishes(ctx, oas.ListPointDishesParams{PointId: n.b.ID})
	requireForbidden(t, err, "catalog.view")

	// блюдо другой компании на своей точке
	foreign := createDish(t, s, "Чужое")
	_, err = n.owner.SetDishOnPoint(ctx, off, oas.SetDishOnPointParams{PointId: n.a.ID, DishId: foreign.ID})
	requireProblem(t, err, http.StatusNotFound)

	_, err = n.owner.SetDishOnPoint(ctx, &oas.DishAvailabilityOnPoint{Enabled: true}, oas.SetDishOnPointParams{PointId: n.a.ID, DishId: pasta.ID})
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"Паста": true, "Суп": true}, enabled(cook, n.a))
}
