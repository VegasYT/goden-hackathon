//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
)

var manager = eventstore.Actor{Type: eventstore.ActorUser, ID: "user-1"}

type fixture struct {
	ctx  context.Context
	pool *pgxpool.Pool
	svc  *catalog.Service
}

func setup(t *testing.T) fixture {
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	runner := command.NewRunner(pool, clk, slog.New(slog.DiscardHandler))
	return fixture{context.Background(), pool, catalog.NewService(runner, pool)}
}

func (f fixture) product(t *testing.T, name string, unit catalog.Unit) catalog.Product {
	t.Helper()
	p, err := f.svc.CreateProduct(f.ctx, manager, "", companyID, catalog.NewProduct{Name: name, Unit: unit, Category: "Тест"})
	require.NoError(t, err)
	return p
}

func (f fixture) dish(t *testing.T, name string) catalog.Dish {
	t.Helper()
	d, err := f.svc.CreateDish(f.ctx, manager, "", companyID, name)
	require.NoError(t, err)
	return d
}

// events читает и восстанавливает события потока.
func (f fixture) events(t *testing.T, stream string) []eventstore.Payload {
	t.Helper()
	stored, err := eventstore.ReadStream(f.ctx, f.pool, stream, 1)
	require.NoError(t, err)
	payloads := make([]eventstore.Payload, len(stored))
	for i, e := range stored {
		payloads[i], err = eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
		require.NoError(t, err)
	}
	return payloads
}

func (f fixture) totalEvents(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM events").Scan(&n))
	return n
}

// Продукты.

func TestCreateProduct(t *testing.T) {
	f := setup(t)
	shelfLife := 5

	created, err := f.svc.CreateProduct(f.ctx, manager, "", companyID, catalog.NewProduct{
		Name: "Сливки 33%", Unit: catalog.UnitMilliliter, Category: "Молочные продукты", ShelfLifeDays: &shelfLife,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)

	read, err := f.svc.Product(f.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created, read)
	require.Equal(t, catalog.Product{
		ID: created.ID, CompanyID: companyID, Name: "Сливки 33%", Unit: catalog.UnitMilliliter,
		Category: "Молочные продукты", ShelfLifeDays: &shelfLife,
	}, read)

	require.Equal(t, []eventstore.Payload{catalog.ProductCreated{
		ProductID: created.ID, CompanyID: companyID, Name: "Сливки 33%", Unit: catalog.UnitMilliliter,
		Category: "Молочные продукты", ShelfLifeDays: &shelfLife,
	}}, f.events(t, catalog.ProductStream(created.ID)))
}

func TestCreateProductWithoutShelfLife(t *testing.T) {
	f := setup(t)

	created := f.product(t, "Соль", catalog.UnitGram)

	read, err := f.svc.Product(f.ctx, created.ID)
	require.NoError(t, err)
	require.Nil(t, read.ShelfLifeDays)
}

func TestCreateProductNameTaken(t *testing.T) {
	f := setup(t)
	f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	before := f.totalEvents(t)

	_, err := f.svc.CreateProduct(f.ctx, manager, "", companyID, catalog.NewProduct{Name: " сливки 33% ", Unit: catalog.UnitGram})

	require.ErrorIs(t, err, catalog.ErrNameTaken)
	require.Equal(t, before, f.totalEvents(t))
}

func TestCreateProductInvalid(t *testing.T) {
	f := setup(t)
	zero := 0

	tests := []struct {
		name  string
		in    catalog.NewProduct
		field string
	}{
		{"empty name", catalog.NewProduct{Name: " ", Unit: catalog.UnitGram}, "name"},
		{"unknown unit", catalog.NewProduct{Name: "Мука", Unit: "kg"}, "unit"},
		{"zero shelf life", catalog.NewProduct{Name: "Мука", Unit: catalog.UnitGram, ShelfLifeDays: &zero}, "shelf_life_days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.svc.CreateProduct(f.ctx, manager, "", companyID, tt.in)

			var validation *catalog.ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, tt.field, validation.Field)
		})
	}
	require.Zero(t, f.totalEvents(t))
}

func TestProductsAreSortedByName(t *testing.T) {
	f := setup(t)
	for _, name := range []string{"Яйцо", "бекон", "Сливки 33%"} {
		f.product(t, name, catalog.UnitGram)
	}

	products, total, err := f.svc.Products(f.ctx, companyID, catalog.Page{Limit: 50})

	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	names := make([]string, len(products))
	for i, p := range products {
		names[i] = p.Name
	}
	require.Equal(t, []string{"бекон", "Сливки 33%", "Яйцо"}, names)

	// вторая страница: общее число не зависит от страницы
	page, total, err := f.svc.Products(f.ctx, companyID, catalog.Page{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, page, 1)
	require.Equal(t, "Яйцо", page[0].Name)
}

func TestProductNotFound(t *testing.T) {
	f := setup(t)

	_, err := f.svc.Product(f.ctx, uuid.Must(uuid.NewV7()))

	require.ErrorIs(t, err, catalog.ErrNotFound)
}

// Блюда.

func TestCreateDish(t *testing.T) {
	f := setup(t)

	created, err := f.svc.CreateDish(f.ctx, manager, "", companyID, "  Паста карбонара ")
	require.NoError(t, err)

	read, err := f.svc.Dish(f.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created, read)
	require.Equal(t, catalog.Dish{ID: created.ID, CompanyID: companyID, Name: "Паста карбонара", Ingredients: []catalog.Ingredient{}}, read)
	require.Equal(t,
		[]eventstore.Payload{catalog.DishCreated{DishID: created.ID, CompanyID: companyID, Name: "Паста карбонара"}},
		f.events(t, catalog.DishStream(created.ID)))
}

func TestCreateDishNameTaken(t *testing.T) {
	f := setup(t)
	f.dish(t, "Паста карбонара")
	before := f.totalEvents(t)

	_, err := f.svc.CreateDish(f.ctx, manager, "", companyID, "ПАСТА КАРБОНАРА")

	require.ErrorIs(t, err, catalog.ErrNameTaken)
	require.Equal(t, before, f.totalEvents(t))
}

func TestDishNotFound(t *testing.T) {
	f := setup(t)

	_, err := f.svc.Dish(f.ctx, uuid.Must(uuid.NewV7()))

	require.ErrorIs(t, err, catalog.ErrNotFound)
}

// Рецептуры.

func TestChangeRecipe(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	bacon := f.product(t, "Бекон", catalog.UnitGram)
	dish := f.dish(t, "Паста карбонара")

	// первая рецептура; чтение возвращает ингредиенты по названию продукта
	changed, err := f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, []catalog.Ingredient{
		{ProductID: cream.ID, Quantity: 80},
		{ProductID: bacon.ID, Quantity: 60},
	})
	require.NoError(t, err)
	want := []catalog.Ingredient{{ProductID: bacon.ID, Quantity: 60}, {ProductID: cream.ID, Quantity: 80}}
	require.Equal(t, want, amounts(changed.Ingredients))
	read, err := f.svc.Dish(f.ctx, dish.ID)
	require.NoError(t, err)
	require.Equal(t, changed, read)

	// новая рецептура заменяет прежнюю целиком
	_, err = f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, []catalog.Ingredient{{ProductID: cream.ID, Quantity: 100}})
	require.NoError(t, err)
	read, err = f.svc.Dish(f.ctx, dish.ID)
	require.NoError(t, err)
	require.Equal(t, []catalog.Ingredient{{ProductID: cream.ID, Quantity: 100}}, amounts(read.Ingredients))

	require.Equal(t, []eventstore.Payload{
		catalog.DishCreated{DishID: dish.ID, CompanyID: companyID, Name: "Паста карбонара"},
		catalog.RecipeChanged{DishID: dish.ID, CompanyID: companyID, Ingredients: []catalog.RecipeIngredient{
			{ProductID: cream.ID, Quantity: 80}, {ProductID: bacon.ID, Quantity: 60},
		}},
		catalog.RecipeChanged{DishID: dish.ID, CompanyID: companyID, Ingredients: []catalog.RecipeIngredient{
			{ProductID: cream.ID, Quantity: 100},
		}},
	}, f.events(t, catalog.DishStream(dish.ID)))
}

func TestChangeRecipeRejected(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	dish := f.dish(t, "Паста карбонара")
	original := []catalog.Ingredient{{ProductID: cream.ID, Quantity: 80}}
	_, err := f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, original)
	require.NoError(t, err)
	before := f.totalEvents(t)
	unknown := uuid.Must(uuid.NewV7())

	t.Run("unknown product", func(t *testing.T) {
		_, err := f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, []catalog.Ingredient{
			{ProductID: cream.ID, Quantity: 80}, {ProductID: unknown, Quantity: 10},
		})

		var unknownProduct *catalog.UnknownProductError
		require.ErrorAs(t, err, &unknownProduct)
		require.Equal(t, unknown, unknownProduct.ProductID)
	})

	invalid := map[string][]catalog.Ingredient{
		"empty":            {},
		"product twice":    {{ProductID: cream.ID, Quantity: 80}, {ProductID: cream.ID, Quantity: 20}},
		"zero quantity":    {{ProductID: cream.ID, Quantity: 0}},
		"negative quanity": {{ProductID: cream.ID, Quantity: -1}},
	}
	for name, ingredients := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, ingredients)

			var validation *catalog.ValidationError
			require.ErrorAs(t, err, &validation)
		})
	}

	t.Run("unknown dish", func(t *testing.T) {
		_, err := f.svc.ChangeRecipe(f.ctx, manager, "", unknown, original)

		require.ErrorIs(t, err, catalog.ErrNotFound)
	})

	read, err := f.svc.Dish(f.ctx, dish.ID)
	require.NoError(t, err)
	require.Equal(t, original, amounts(read.Ingredients), "recipe must stay unchanged")
	require.Equal(t, before, f.totalEvents(t))
}

func TestDishesWithRecipes(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	soup := f.dish(t, "Суп")
	pasta := f.dish(t, "Паста")
	_, err := f.svc.ChangeRecipe(f.ctx, manager, "", pasta.ID, []catalog.Ingredient{{ProductID: cream.ID, Quantity: 80}})
	require.NoError(t, err)

	dishes, total, err := f.svc.Dishes(f.ctx, companyID, catalog.ListFilter{}, catalog.Page{Limit: 50})

	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []catalog.Dish{
		{ID: pasta.ID, CompanyID: companyID, Name: "Паста", Ingredients: []catalog.Ingredient{
			{ProductID: cream.ID, Quantity: 80, ProductName: "Сливки 33%", Unit: catalog.UnitMilliliter},
		}},
		{ID: soup.ID, CompanyID: companyID, Name: "Суп", Ingredients: []catalog.Ingredient{}},
	}, dishes)

	// страница: рецептуры читаются только для её блюд, общее число не меняется
	first, total, err := f.svc.Dishes(f.ctx, companyID, catalog.ListFilter{}, catalog.Page{Limit: 1})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []catalog.Dish{dishes[0]}, first)

	second, _, err := f.svc.Dishes(f.ctx, companyID, catalog.ListFilter{}, catalog.Page{Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Equal(t, []catalog.Dish{dishes[1]}, second)
}

// Параллельность и идемпотентность.

func TestConcurrentRecipeChanges(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	dish := f.dish(t, "Паста карбонара")

	var g errgroup.Group
	for _, quantity := range []int64{80, 100} {
		g.Go(func() error {
			_, err := f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, []catalog.Ingredient{{ProductID: cream.ID, Quantity: quantity}})
			return err
		})
	}
	require.NoError(t, g.Wait())

	events := f.events(t, catalog.DishStream(dish.ID))
	require.Len(t, events, 3, "DishCreated and two RecipeChanged")
	last := events[2].(catalog.RecipeChanged)
	read, err := f.svc.Dish(f.ctx, dish.ID)
	require.NoError(t, err)
	require.Equal(t, []catalog.Ingredient{{ProductID: cream.ID, Quantity: last.Ingredients[0].Quantity}}, amounts(read.Ingredients),
		"the table must match the last event of the stream")
}

func TestConcurrentProductsWithSameName(t *testing.T) {
	f := setup(t)

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := f.svc.CreateProduct(f.ctx, manager, "", companyID, catalog.NewProduct{Name: "Бекон", Unit: catalog.UnitGram})
			errs <- err
		}()
	}
	first, second := <-errs, <-errs

	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, catalog.ErrNameTaken)
	products, _, err := f.svc.Products(f.ctx, companyID, catalog.Page{Limit: 50})
	require.NoError(t, err)
	require.Len(t, products, 1)
	require.Equal(t, 1, f.totalEvents(t))
}

func TestCreateProductIsIdempotent(t *testing.T) {
	f := setup(t)
	in := catalog.NewProduct{Name: "Бекон", Unit: catalog.UnitGram, Category: "Мясо"}

	first, err := f.svc.CreateProduct(f.ctx, manager, "key-1", companyID, in)
	require.NoError(t, err)
	second, err := f.svc.CreateProduct(f.ctx, manager, "key-1", companyID, in)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, 1, f.totalEvents(t))

	// тот же ключ с другими данными - ошибка, а не тихий возврат чужого результата
	in.Category = "Копчёности"
	_, err = f.svc.CreateProduct(f.ctx, manager, "key-1", companyID, in)
	require.True(t, errors.Is(err, command.ErrKeyReused))
}

func TestUnitNeverChanges(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	dish := f.dish(t, "Паста карбонара")

	for _, quantity := range []int64{80, 90, 100} {
		_, err := f.svc.ChangeRecipe(f.ctx, manager, "", dish.ID, []catalog.Ingredient{{ProductID: cream.ID, Quantity: quantity}})
		require.NoError(t, err)
	}

	read, err := f.svc.Product(f.ctx, cream.ID)
	require.NoError(t, err)
	require.Equal(t, catalog.UnitMilliliter, read.Unit)
}

// Команда другого модуля читает блюдо в своей транзакции и видит изменения этой транзакции.
func TestDishInReadsWithinTransaction(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	dish := f.dish(t, "Паста карбонара")

	tx, err := f.pool.Begin(f.ctx)
	require.NoError(t, err)
	defer tx.Rollback(f.ctx) //nolint:errcheck
	_, err = tx.Exec(f.ctx, "INSERT INTO catalog_recipe_ingredients VALUES ($1, $2, 80)", dish.ID, cream.ID)
	require.NoError(t, err)

	inside, err := f.svc.DishIn(f.ctx, tx, dish.ID)
	require.NoError(t, err)
	require.Equal(t, []catalog.Ingredient{{ProductID: cream.ID, Quantity: 80}}, amounts(inside.Ingredients))

	outside, err := f.svc.Dish(f.ctx, dish.ID)
	require.NoError(t, err)
	require.Empty(t, outside.Ingredients, "the change is not committed yet")
}

// Каталог принадлежит компании: названия, списки и рецептуры одной компании не пересекаются с другой.
func TestCatalogIsSeparatePerCompany(t *testing.T) {
	f := setup(t)
	other := uuid.Must(uuid.NewV7())
	cream := f.product(t, "Сливки 33%", catalog.UnitMilliliter)
	f.product(t, "Бекон", catalog.UnitGram)
	pasta := f.dish(t, "Паста")

	// то же название в другой компании свободно
	foreignCream, err := f.svc.CreateProduct(f.ctx, manager, "", other, catalog.NewProduct{Name: "сливки 33%", Unit: catalog.UnitGram})
	require.NoError(t, err)
	require.Equal(t, other, foreignCream.CompanyID)
	foreignPasta, err := f.svc.CreateDish(f.ctx, manager, "", other, "Паста")
	require.NoError(t, err)

	products, total, err := f.svc.Products(f.ctx, other, catalog.Page{Limit: 50})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []catalog.Product{foreignCream}, products)
	dishes, total, err := f.svc.Dishes(f.ctx, companyID, catalog.ListFilter{}, catalog.Page{Limit: 50})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, pasta.ID, dishes[0].ID)

	// по идентификатору чужой каталог не читается
	_, err = f.svc.ProductOf(f.ctx, companyID, foreignCream.ID)
	require.ErrorIs(t, err, catalog.ErrNotFound)
	_, err = f.svc.DishOf(f.ctx, companyID, foreignPasta.ID)
	require.ErrorIs(t, err, catalog.ErrNotFound)
	own, err := f.svc.ProductOf(f.ctx, companyID, cream.ID)
	require.NoError(t, err)
	require.Equal(t, cream, own)

	// продукт другой компании в рецептуре - неизвестный продукт
	_, err = f.svc.ChangeRecipe(f.ctx, manager, "", pasta.ID, []catalog.Ingredient{{ProductID: foreignCream.ID, Quantity: 10}})
	var unknown *catalog.UnknownProductError
	require.ErrorAs(t, err, &unknown)
	require.Equal(t, foreignCream.ID, unknown.ProductID)
	read, err := f.svc.Dish(f.ctx, pasta.ID)
	require.NoError(t, err)
	require.Empty(t, read.Ingredients)
}

func TestDishOnPoint(t *testing.T) {
	f := setup(t)
	a, b := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	pasta, soup := f.dish(t, "Паста"), f.dish(t, "Суп")

	enabled := func(point uuid.UUID) map[string]bool {
		dishes, total, err := f.svc.PointDishes(f.ctx, companyID, point, catalog.Page{Limit: 50})
		require.NoError(t, err)
		require.EqualValues(t, len(dishes), total)
		out := map[string]bool{}
		for _, d := range dishes {
			out[d.Name] = d.Enabled
		}
		return out
	}
	require.Equal(t, map[string]bool{"Паста": true, "Суп": true}, enabled(a), "a dish is sold on every point until disabled")

	changed, err := f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, a, pasta.ID, false)
	require.NoError(t, err)
	require.Equal(t, catalog.PointDish{DishID: pasta.ID, Name: "Паста", Enabled: false}, changed)
	require.Equal(t, map[string]bool{"Паста": false, "Суп": true}, enabled(a))
	require.Equal(t, map[string]bool{"Паста": true, "Суп": true}, enabled(b), "points are independent")

	// повтор действующего состояния события не пишет
	_, err = f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, a, pasta.ID, false)
	require.NoError(t, err)
	_, err = f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, a, soup.ID, true)
	require.NoError(t, err)
	require.Equal(t, []eventstore.Payload{
		catalog.DishAvailabilityChanged{PointID: a, DishID: pasta.ID, Enabled: false},
	}, f.events(t, catalog.MenuStream(a)))

	_, err = f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, a, pasta.ID, true)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"Паста": true, "Суп": true}, enabled(a))

	// выключенное блюдо не входит в блюда, которые продаются на точке
	_, err = f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, b, soup.ID, false)
	require.NoError(t, err)
	sold, total, err := f.svc.DishesOnPoint(f.ctx, companyID, b, catalog.Page{Limit: 50})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, pasta.ID, sold[0].ID)
	disabled, err := f.svc.DisabledOnPoint(f.ctx, f.pool, b, []uuid.UUID{pasta.ID, soup.ID})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{soup.ID}, disabled)

	// новое блюдо включено на всех точках
	f.dish(t, "Омлет")
	require.True(t, enabled(a)["Омлет"])

	// блюдо другой компании и несуществующее блюдо
	foreign, err := f.svc.CreateDish(f.ctx, manager, "", uuid.Must(uuid.NewV7()), "Чужое")
	require.NoError(t, err)
	_, err = f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, a, foreign.ID, false)
	require.ErrorIs(t, err, catalog.ErrNotFound)
	_, err = f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, a, uuid.Must(uuid.NewV7()), false)
	require.ErrorIs(t, err, catalog.ErrNotFound)
	require.Len(t, f.events(t, catalog.MenuStream(a)), 2)
}

// amounts - рецептура без данных продукта, которые подставляет чтение: только продукты и количества.
func amounts(ingredients []catalog.Ingredient) []catalog.Ingredient {
	out := make([]catalog.Ingredient, len(ingredients))
	for i, ingredient := range ingredients {
		out[i] = catalog.Ingredient{ProductID: ingredient.ProductID, Quantity: ingredient.Quantity}
	}
	return out
}
