package generator

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"goden/backend/internal/api/oas"
)

type seedProduct struct {
	name          string
	unit          oas.Unit
	category      string
	shelfLifeDays int32
}

type seedIngredient struct {
	product  string // название продукта из seedProducts
	quantity int64  // на одну порцию
}

type seedDish struct {
	name        string
	ingredients []seedIngredient
}

// Демонстрационный каталог: названия - данные интерфейса, поэтому на русском.
var (
	seedProducts = []seedProduct{
		{"Сливки 33%", oas.UnitMl, "Молочные продукты", 7},
		{"Молоко", oas.UnitMl, "Молочные продукты", 5},
		{"Сыр пармезан", oas.UnitG, "Молочные продукты", 30},
		{"Бекон", oas.UnitG, "Мясо", 10},
		{"Куриное филе", oas.UnitG, "Мясо", 4},
		{"Яйцо куриное", oas.UnitPcs, "Яйца", 20},
		{"Спагетти", oas.UnitG, "Бакалея", 365},
		{"Томаты", oas.UnitG, "Овощи", 6},
		{"Салат романо", oas.UnitG, "Овощи", 4},
		{"Хлеб пшеничный", oas.UnitG, "Выпечка", 3},
		{"Масло оливковое", oas.UnitMl, "Бакалея", 365},
	}
	seedDishes = []seedDish{
		{"Паста карбонара", []seedIngredient{{"Спагетти", 120}, {"Бекон", 60}, {"Сливки 33%", 80}, {"Яйцо куриное", 1}, {"Сыр пармезан", 20}}},
		{"Салат Цезарь с курицей", []seedIngredient{{"Салат романо", 100}, {"Куриное филе", 120}, {"Сыр пармезан", 15}, {"Хлеб пшеничный", 30}, {"Масло оливковое", 15}}},
		{"Омлет", []seedIngredient{{"Яйцо куриное", 3}, {"Молоко", 50}, {"Масло оливковое", 5}}},
		{"Брускетта с томатами", []seedIngredient{{"Хлеб пшеничный", 80}, {"Томаты", 100}, {"Масло оливковое", 10}}},
		{"Куриное филе в сливочном соусе", []seedIngredient{{"Куриное филе", 180}, {"Сливки 33%", 60}}},
	}
)

// Seed наполняет каталог компании демонстрационным набором, если каталог пуст либо набор
// в нём начат и не закончен. Каталог, в котором есть что-то своё, не меняется.
//
// Незаконченный набор определяется по самому каталогу, а не по памяти процесса: генератор
// могли перезапустить посреди наполнения, и без этого каталог остался бы без блюд навсегда.
func (s *Simulator) Seed(ctx context.Context, companyID uuid.UUID) error {
	products, err := s.api.ListProducts(ctx, oas.ListProductsParams{CompanyId: companyID, Limit: oas.NewOptInt32(pageSize)})
	if err != nil {
		return err
	}
	dishes, err := s.api.ListDishes(ctx, oas.ListDishesParams{CompanyId: companyID, Limit: oas.NewOptInt32(pageSize)})
	if err != nil {
		return err
	}
	if !seedNeeded(products, dishes) {
		return nil
	}
	// архив - дело владельца: такой каталог не наполняется, иначе генератор пытался бы
	// заново создать архивный продукт или блюдо набора и упирался в занятое название
	if archived, err := s.catalogHasArchive(ctx, companyID); err != nil || archived {
		return err
	}

	// уже созданное берётся по названию: наполнение продолжается с места остановки
	ids := make(map[string]uuid.UUID, len(seedProducts))
	for _, product := range products.Items {
		ids[product.Name] = product.ID
	}
	for i, p := range seedProducts {
		if _, found := ids[p.name]; found {
			continue
		}
		product, err := s.api.CreateProduct(ctx, &oas.NewProduct{
			Name: p.name, Unit: p.unit, Category: oas.NewOptString(p.category), ShelfLifeDays: oas.NewOptInt32(p.shelfLifeDays),
		}, oas.CreateProductParams{CompanyId: companyID, IdempotencyKey: seedKey(companyID, "product", i)})
		if err != nil {
			return fmt.Errorf("generator: seed product %q: %w", p.name, err)
		}
		ids[p.name] = product.ID
	}

	existing := make(map[string]oas.Dish, len(dishes.Items))
	for _, dish := range dishes.Items {
		existing[dish.Name] = dish
	}
	for i, d := range seedDishes {
		dish, found := existing[d.name]
		if found && len(dish.Ingredients) > 0 {
			continue
		}
		if !found {
			created, err := s.api.CreateDish(ctx, &oas.NewDish{Name: d.name}, oas.CreateDishParams{CompanyId: companyID, IdempotencyKey: seedKey(companyID, "dish", i)})
			if err != nil {
				return fmt.Errorf("generator: seed dish %q: %w", d.name, err)
			}
			dish = *created
		}
		recipe := &oas.Recipe{}
		for _, in := range d.ingredients {
			recipe.Ingredients = append(recipe.Ingredients, oas.Ingredient{ProductID: ids[in.product], Quantity: in.quantity})
		}
		_, err = s.api.ChangeRecipe(ctx, recipe, oas.ChangeRecipeParams{CompanyId: companyID, DishId: dish.ID, IdempotencyKey: seedKey(companyID, "recipe", i)})
		if err != nil {
			return fmt.Errorf("generator: seed recipe of %q: %w", d.name, err)
		}
	}
	s.logger.InfoContext(ctx, "generator: catalog seeded", "products", len(seedProducts), "dishes", len(seedDishes))
	return nil
}

// catalogHasArchive сообщает, есть ли в каталоге компании архивные продукты или блюда.
func (s *Simulator) catalogHasArchive(ctx context.Context, companyID uuid.UUID) (bool, error) {
	archived := oas.NewOptCatalogStatus(oas.CatalogStatusArchived)
	products, err := s.api.ListProducts(ctx, oas.ListProductsParams{CompanyId: companyID, Limit: oas.NewOptInt32(1), Status: archived})
	if err != nil {
		return false, err
	}
	dishes, err := s.api.ListDishes(ctx, oas.ListDishesParams{CompanyId: companyID, Limit: oas.NewOptInt32(1), Status: archived})
	if err != nil {
		return false, err
	}
	return products.Total > 0 || dishes.Total > 0, nil
}

// seedNeeded решает, наполнять ли каталог: пустой - да; с продуктом или блюдом не из набора -
// нет, это каталог владельца; состоящий только из набора - да, пока в нём чего-то не хватает.
func seedNeeded(products *oas.ProductPage, dishes *oas.DishPage) bool {
	if products.Total == 0 && dishes.Total == 0 {
		return true
	}
	// не вся страница - в каталоге заведомо больше, чем набор
	if products.Total > int64(len(products.Items)) || dishes.Total > int64(len(dishes.Items)) {
		return false
	}
	known := make(map[string]bool, len(seedProducts)+len(seedDishes))
	for _, p := range seedProducts {
		known["product "+p.name] = true
	}
	for _, d := range seedDishes {
		known["dish "+d.name] = true
	}
	complete := len(products.Items) == len(seedProducts) && len(dishes.Items) == len(seedDishes)
	for _, product := range products.Items {
		if !known["product "+product.Name] {
			return false
		}
	}
	for _, dish := range dishes.Items {
		if !known["dish "+dish.Name] {
			return false
		}
		complete = complete && len(dish.Ingredients) > 0
	}
	return !complete
}

// Ключ включает компанию: ключи идемпотентности общие на систему, а каталог у каждой компании свой.
func seedKey(companyID uuid.UUID, kind string, i int) oas.OptString {
	return oas.NewOptString(fmt.Sprintf("seed-%s-%s-%d", companyID, kind, i))
}
