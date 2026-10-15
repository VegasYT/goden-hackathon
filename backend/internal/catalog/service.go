package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/catalog/db"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

// Service выполняет команды каталога и читает его данные.
type Service struct {
	runner *command.Runner
	pool   *pgxpool.Pool
	usage  ProductUsage
}

func NewService(runner *command.Runner, pool *pgxpool.Pool) *Service {
	return &Service{runner: runner, pool: pool}
}

// ProductUsage сообщает, какие продукты уже были в поставках или на складе: у них нельзя менять
// единицу. Реализует inventory, каталог его не импортирует. Читает через переданное соединение,
// чтобы команда проверяла в своей транзакции.
type ProductUsage interface {
	DeliveredProducts(ctx context.Context, q Querier, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

// SetProductUsage подключает проверку поставок и склада. Без неё единица считается
// занятой только рецептурами.
func (s *Service) SetProductUsage(usage ProductUsage) { s.usage = usage }

// CreateProduct создаёт продукт в компании. key - ключ идемпотентности, пустой отключает её.
// Существование компании не проверяется: право вызывающего в ней проверил слой API.
func (s *Service) CreateProduct(ctx context.Context, actor eventstore.Actor, key string, companyID uuid.UUID, in NewProduct) (Product, error) {
	in, err := in.normalized()
	if err != nil {
		return Product{}, err
	}
	product := Product{
		ID: uuid.Must(uuid.NewV7()), CompanyID: companyID, Name: in.Name, Unit: in.Unit,
		Category: in.Category, ShelfLifeDays: in.ShelfLifeDays,
	}

	spec := command.Spec{
		Name: "CreateProduct", Actor: actor, IdempotencyKey: key,
		Request: struct {
			CompanyID uuid.UUID
			Product   NewProduct
		}{companyID, in},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Product, error) {
		q := db.New(tx)
		if err := productNameFree(ctx, q, companyID, product.Name, uuid.Nil); err != nil {
			return Product{}, err
		}

		event := ProductCreated{
			ProductID: product.ID, CompanyID: companyID, Name: product.Name, Unit: product.Unit,
			Category: product.Category, ShelfLifeDays: product.ShelfLifeDays,
		}
		if _, err := tx.Append(ctx, ProductStream(product.ID), 0, event); err != nil {
			return Product{}, err
		}
		if err := applyProductCreated(ctx, q, event); err != nil {
			return Product{}, err
		}
		return product, nil
	})
}

// CreateDish создаёт в компании блюдо. Рецептуру можно передать сразу: тогда она записывается
// в той же команде, и блюдо ни мгновения не бывает без неё.
func (s *Service) CreateDish(
	ctx context.Context, actor eventstore.Actor, key string, companyID uuid.UUID, name string, ingredients ...Ingredient,
) (Dish, error) {
	name, err := normalizedDishName(name)
	if err != nil {
		return Dish{}, err
	}
	if len(ingredients) > 0 {
		if err := validateRecipe(ingredients); err != nil {
			return Dish{}, err
		}
	}
	dishID := uuid.Must(uuid.NewV7())

	spec := command.Spec{
		Name: "CreateDish", Actor: actor, IdempotencyKey: key,
		Request: struct {
			CompanyID   uuid.UUID
			Name        string
			Ingredients []Ingredient `json:",omitempty"`
		}{companyID, name, ingredients},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Dish, error) {
		q := db.New(tx)
		if err := dishNameFree(ctx, q, companyID, name, uuid.Nil); err != nil {
			return Dish{}, err
		}

		stream := DishStream(dishID)
		created := DishCreated{DishID: dishID, CompanyID: companyID, Name: name}
		if _, err := tx.Append(ctx, stream, 0, created); err != nil {
			return Dish{}, err
		}
		if err := applyDishCreated(ctx, q, created); err != nil {
			return Dish{}, err
		}
		if len(ingredients) > 0 {
			if err := requireProducts(ctx, q, companyID, ingredients); err != nil {
				return Dish{}, err
			}
			recipe := recipeChanged(dishID, companyID, ingredients)
			if _, err := tx.Append(ctx, stream, 1, recipe); err != nil {
				return Dish{}, err
			}
			if err := applyRecipeChanged(ctx, q, recipe); err != nil {
				return Dish{}, err
			}
		}
		return readDish(ctx, q, dishID)
	})
}

func recipeChanged(dishID, companyID uuid.UUID, ingredients []Ingredient) RecipeChanged {
	event := RecipeChanged{DishID: dishID, CompanyID: companyID, Ingredients: make([]RecipeIngredient, len(ingredients))}
	for i, ing := range ingredients {
		event.Ingredients[i] = RecipeIngredient{ProductID: ing.ProductID, Quantity: ing.Quantity}
	}
	return event
}

// ChangeRecipe заменяет рецептуру блюда целиком.
func (s *Service) ChangeRecipe(ctx context.Context, actor eventstore.Actor, key string, dishID uuid.UUID, ingredients []Ingredient) (Dish, error) {
	if err := validateRecipe(ingredients); err != nil {
		return Dish{}, err
	}

	spec := command.Spec{
		Name: "ChangeRecipe", Actor: actor, IdempotencyKey: key,
		// Блокировка потока блюда: с ключом идемпотентности транзакция получает номер до чтения
		// версии потока, и две одновременные смены рецептуры могли записать версии не в том
		// порядке, в каком их увидят читатели событий (ADR-002).
		Locks: []string{DishStream(dishID)},
		Request: struct {
			DishID      uuid.UUID
			Ingredients []Ingredient
		}{dishID, ingredients},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Dish, error) {
		q := db.New(tx)
		row, err := q.GetDish(ctx, dishID)
		if err != nil {
			return Dish{}, notFound(err, "dish", dishID)
		}
		if err := requireProducts(ctx, q, row.CompanyID, ingredients); err != nil {
			return Dish{}, err
		}

		stream := DishStream(dishID)
		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return Dish{}, err
		}
		event := recipeChanged(dishID, row.CompanyID, ingredients)
		if _, err := tx.Append(ctx, stream, version, event); err != nil {
			return Dish{}, err
		}
		if err := applyRecipeChanged(ctx, q, event); err != nil {
			return Dish{}, err
		}
		return readDish(ctx, q, dishID)
	})
}

// ChangeProduct меняет продукт: переданные поля меняются, остальные сохраняются. Единица
// меняется, только пока продукт нигде не использовался. Изменение без отличий события не пишет.
func (s *Service) ChangeProduct(
	ctx context.Context, actor eventstore.Actor, key string, productID uuid.UUID, change ProductChange,
) (Product, error) {
	stream := ProductStream(productID)
	spec := command.Spec{
		Name: "ChangeProduct", Actor: actor, IdempotencyKey: key,
		// продукт меняют не первым событием потока: блокировка держит версии по порядку
		Locks: []string{stream},
		Request: struct {
			ProductID uuid.UUID
			Change    ProductChange
		}{productID, change},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Product, error) {
		q := db.New(tx)
		current, err := readProduct(ctx, q, productID)
		if err != nil {
			return Product{}, err
		}
		next, err := change.applied(current).normalized()
		if err != nil {
			return Product{}, err
		}
		if next.sameAs(current) {
			return current, nil
		}
		if next.Name != current.Name {
			if err := productNameFree(ctx, q, current.CompanyID, next.Name, productID); err != nil {
				return Product{}, err
			}
		}
		if next.Unit != current.Unit {
			locked, err := s.unitLocked(ctx, tx, q, []uuid.UUID{productID})
			if err != nil {
				return Product{}, err
			}
			if locked[productID] {
				return Product{}, fmt.Errorf("product %s: %w", productID, ErrUnitLocked)
			}
		}

		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return Product{}, err
		}
		event := ProductChanged{
			ProductID: productID, CompanyID: current.CompanyID, Name: next.Name, Unit: next.Unit,
			Category: next.Category, ShelfLifeDays: next.ShelfLifeDays,
		}
		if _, err := tx.Append(ctx, stream, version, event); err != nil {
			return Product{}, err
		}
		if err := applyProductChanged(ctx, q, event); err != nil {
			return Product{}, err
		}
		return readProduct(ctx, q, productID)
	})
}

// ArchiveProduct отправляет продукт в архив. Продукт из рецептуры неархивного блюда
// в архив не уходит: ProductInUseError называет эти блюда.
func (s *Service) ArchiveProduct(ctx context.Context, actor eventstore.Actor, key string, productID uuid.UUID) (Product, error) {
	return s.setProductArchived(ctx, actor, key, productID, true)
}

// RestoreProduct возвращает продукт из архива.
func (s *Service) RestoreProduct(ctx context.Context, actor eventstore.Actor, key string, productID uuid.UUID) (Product, error) {
	return s.setProductArchived(ctx, actor, key, productID, false)
}

func (s *Service) setProductArchived(
	ctx context.Context, actor eventstore.Actor, key string, productID uuid.UUID, archived bool,
) (Product, error) {
	stream := ProductStream(productID)
	name := "RestoreProduct"
	if archived {
		name = "ArchiveProduct"
	}
	spec := command.Spec{Name: name, Actor: actor, IdempotencyKey: key, Locks: []string{stream}, Request: productID}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Product, error) {
		q := db.New(tx)
		product, err := readProduct(ctx, q, productID)
		if err != nil {
			return Product{}, err
		}
		if product.Archived == archived {
			return product, nil
		}
		var event eventstore.Payload = ProductRestored{ProductID: productID, CompanyID: product.CompanyID}
		if archived {
			dishes, err := q.DishesWithProduct(ctx, productID)
			if err != nil {
				return Product{}, fmt.Errorf("catalog: read dishes with product: %w", err)
			}
			if len(dishes) > 0 {
				names := make([]string, len(dishes))
				for i, dish := range dishes {
					names[i] = dish.Name
				}
				return Product{}, &ProductInUseError{Dishes: names}
			}
			event = ProductArchived{ProductID: productID, CompanyID: product.CompanyID}
		}

		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return Product{}, err
		}
		if _, err := tx.Append(ctx, stream, version, event); err != nil {
			return Product{}, err
		}
		if err := applyProductArchived(ctx, q, productID, archived); err != nil {
			return Product{}, err
		}
		product.Archived = archived
		return product, nil
	})
}

// RenameDish переименовывает блюдо. Переименование в текущее название события не пишет.
func (s *Service) RenameDish(ctx context.Context, actor eventstore.Actor, key string, dishID uuid.UUID, name string) (Dish, error) {
	name, err := normalizedDishName(name)
	if err != nil {
		return Dish{}, err
	}
	stream := DishStream(dishID)
	spec := command.Spec{
		Name: "RenameDish", Actor: actor, IdempotencyKey: key, Locks: []string{stream},
		Request: struct {
			DishID uuid.UUID
			Name   string
		}{dishID, name},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Dish, error) {
		q := db.New(tx)
		row, err := q.GetDish(ctx, dishID)
		if err != nil {
			return Dish{}, notFound(err, "dish", dishID)
		}
		if row.Name == name {
			return readDish(ctx, q, dishID)
		}
		if err := dishNameFree(ctx, q, row.CompanyID, name, dishID); err != nil {
			return Dish{}, err
		}
		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return Dish{}, err
		}
		event := DishRenamed{DishID: dishID, CompanyID: row.CompanyID, Name: name}
		if _, err := tx.Append(ctx, stream, version, event); err != nil {
			return Dish{}, err
		}
		if err := applyDishRenamed(ctx, q, event); err != nil {
			return Dish{}, err
		}
		return readDish(ctx, q, dishID)
	})
}

// ArchiveDish отправляет блюдо в архив: оно не продаётся ни на одной точке компании.
func (s *Service) ArchiveDish(ctx context.Context, actor eventstore.Actor, key string, dishID uuid.UUID) (Dish, error) {
	return s.setDishArchived(ctx, actor, key, dishID, true)
}

// RestoreDish возвращает блюдо из архива. Блюдо с архивным продуктом в рецептуре
// не восстанавливается: ArchivedIngredientsError называет эти продукты.
func (s *Service) RestoreDish(ctx context.Context, actor eventstore.Actor, key string, dishID uuid.UUID) (Dish, error) {
	return s.setDishArchived(ctx, actor, key, dishID, false)
}

func (s *Service) setDishArchived(
	ctx context.Context, actor eventstore.Actor, key string, dishID uuid.UUID, archived bool,
) (Dish, error) {
	stream := DishStream(dishID)
	name := "RestoreDish"
	if archived {
		name = "ArchiveDish"
	}
	spec := command.Spec{Name: name, Actor: actor, IdempotencyKey: key, Locks: []string{stream}, Request: dishID}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Dish, error) {
		q := db.New(tx)
		row, err := q.GetDish(ctx, dishID)
		if err != nil {
			return Dish{}, notFound(err, "dish", dishID)
		}
		if row.Archived == archived {
			return readDish(ctx, q, dishID)
		}
		var event eventstore.Payload = DishArchived{DishID: dishID, CompanyID: row.CompanyID}
		if !archived {
			products, err := q.ArchivedProductsOfDish(ctx, dishID)
			if err != nil {
				return Dish{}, fmt.Errorf("catalog: read archived ingredients: %w", err)
			}
			if len(products) > 0 {
				return Dish{}, &ArchivedIngredientsError{Products: products}
			}
			event = DishRestored{DishID: dishID, CompanyID: row.CompanyID}
		}

		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return Dish{}, err
		}
		if _, err := tx.Append(ctx, stream, version, event); err != nil {
			return Dish{}, err
		}
		if err := applyDishArchived(ctx, q, dishID, archived); err != nil {
			return Dish{}, err
		}
		return readDish(ctx, q, dishID)
	})
}

// unitLocked отвечает по каждому продукту, нельзя ли уже сменить его единицу: продукт есть
// в рецептуре любого блюда, в том числе архивного, или был в поставке.
func (s *Service) unitLocked(ctx context.Context, conn Querier, q *db.Queries, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	locked := map[uuid.UUID]bool{}
	for _, id := range ids {
		use, err := q.ProductRecipeUse(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("catalog: read product recipes: %w", err)
		}
		locked[id] = use.InRecipe
	}
	return s.withDelivered(ctx, conn, locked)
}

// withDelivered добавляет к занятым единицам продукты, которые были в поставках.
func (s *Service) withDelivered(ctx context.Context, conn Querier, locked map[uuid.UUID]bool) (map[uuid.UUID]bool, error) {
	if s.usage == nil {
		return locked, nil
	}
	var free []uuid.UUID
	for id, l := range locked {
		if !l {
			free = append(free, id)
		}
	}
	if len(free) == 0 {
		return locked, nil
	}
	delivered, err := s.usage.DeliveredProducts(ctx, conn, free)
	if err != nil {
		return nil, err
	}
	for id := range delivered {
		locked[id] = locked[id] || delivered[id]
	}
	return locked, nil
}

// Product возвращает продукт или ErrNotFound.
func (s *Service) Product(ctx context.Context, id uuid.UUID) (Product, error) {
	return readProduct(ctx, db.New(s.pool), id)
}

// ActiveProductOf возвращает неархивный продукт компании: только такой можно взять в новую поставку.
// Архивный - ArchivedProductError, чужой и несуществующий - ErrNotFound.
func (s *Service) ActiveProductOf(ctx context.Context, companyID, id uuid.UUID) (Product, error) {
	product, err := s.ProductOf(ctx, companyID, id)
	if err == nil && product.Archived {
		return Product{}, &ArchivedProductError{ProductID: id, Name: product.Name}
	}
	return product, err
}

// DishName - блюдо в списке, где нужно только название.
type DishName struct {
	ID   uuid.UUID
	Name string
}

// ProductUse возвращает, где используется продукт: неархивные блюда с ним в рецептуре
// и можно ли ещё менять его единицу.
func (s *Service) ProductUse(ctx context.Context, id uuid.UUID) (ProductUse, []DishName, error) {
	q := db.New(s.pool)
	rows, err := q.DishesWithProduct(ctx, id)
	if err != nil {
		return ProductUse{}, nil, fmt.Errorf("catalog: read dishes with product: %w", err)
	}
	dishes := make([]DishName, len(rows))
	for i, row := range rows {
		dishes[i] = DishName{ID: row.ID, Name: row.Name}
	}
	locked, err := s.unitLocked(ctx, s.pool, q, []uuid.UUID{id})
	if err != nil {
		return ProductUse{}, nil, err
	}
	return ProductUse{Dishes: int64(len(dishes)), UnitLocked: locked[id]}, dishes, nil
}

// Category - категория продуктов компании и число неархивных продуктов в ней.
type Category struct {
	Name     string
	Products int64
}

// ProductCategories возвращает непустые категории продуктов компании по названию.
func (s *Service) ProductCategories(ctx context.Context, companyID uuid.UUID) ([]Category, error) {
	rows, err := db.New(s.pool).ProductCategories(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list categories: %w", err)
	}
	categories := make([]Category, len(rows))
	for i, row := range rows {
		categories[i] = Category{Name: row.Category, Products: row.Products}
	}
	return categories, nil
}

// ProductOf возвращает продукт компании. Продукт другой компании - ErrNotFound:
// по идентификатору нельзя прочитать чужой каталог.
func (s *Service) ProductOf(ctx context.Context, companyID, id uuid.UUID) (Product, error) {
	product, err := s.Product(ctx, id)
	if err == nil && product.CompanyID != companyID {
		return Product{}, fmt.Errorf("product %s: %w", id, ErrNotFound)
	}
	return product, err
}

// DishOf возвращает блюдо компании с рецептурой. Блюдо другой компании - ErrNotFound.
func (s *Service) DishOf(ctx context.Context, companyID, id uuid.UUID) (Dish, error) {
	dish, err := s.Dish(ctx, id)
	if err == nil && dish.CompanyID != companyID {
		return Dish{}, fmt.Errorf("dish %s: %w", id, ErrNotFound)
	}
	return dish, err
}

// ProductsByID возвращает продукты с перечисленными идентификаторами. Неизвестные идентификаторы
// в результат не попадают.
func (s *Service) ProductsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Product, error) {
	rows, err := db.New(s.pool).GetProductsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("catalog: read products: %w", err)
	}
	products := make(map[uuid.UUID]Product, len(rows))
	for _, row := range rows {
		products[row.ID] = productFromRow(row.ID, row.CompanyID, row.Name, row.Unit, row.Category, row.ShelfLifeDays, row.Archived)
	}
	return products, nil
}

// Page - страница списка.
type Page struct {
	Limit  int32
	Offset int32
}

// Products возвращает страницу неархивных продуктов компании по алфавиту и их общее число.
func (s *Service) Products(ctx context.Context, companyID uuid.UUID, page Page) ([]Product, int64, error) {
	listed, total, err := s.ListProducts(ctx, companyID, ListFilter{}, page)
	if err != nil {
		return nil, 0, err
	}
	products := make([]Product, len(listed))
	for i, item := range listed {
		products[i] = item.Product
	}
	return products, total, nil
}

// ListedProduct - продукт списка каталога вместе с тем, где он используется.
type ListedProduct struct {
	Product
	Use ProductUse
}

// ListProducts возвращает страницу продуктов компании под фильтры и их общее число.
func (s *Service) ListProducts(ctx context.Context, companyID uuid.UUID, filter ListFilter, page Page) ([]ListedProduct, int64, error) {
	q := db.New(s.pool)
	part := namePart(filter.Query)
	rows, err := q.ListProducts(ctx, db.ListProductsParams{
		CompanyID: companyID, Archived: filter.Archived, NamePart: part, Category: filter.Category,
		ByCategory: filter.ByCategory, PageLimit: page.Limit, PageOffset: page.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list products: %w", err)
	}
	total, err := q.CountProducts(ctx, db.CountProductsParams{
		CompanyID: companyID, Archived: filter.Archived, NamePart: part, Category: filter.Category,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: count products: %w", err)
	}
	locked := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		locked[row.ID] = row.InRecipe
	}
	if locked, err = s.withDelivered(ctx, s.pool, locked); err != nil {
		return nil, 0, err
	}
	products := make([]ListedProduct, len(rows))
	for i, row := range rows {
		products[i] = ListedProduct{
			Product: productFromRow(row.ID, row.CompanyID, row.Name, row.Unit, row.Category, row.ShelfLifeDays, row.Archived),
			Use:     ProductUse{Dishes: row.Dishes, UnitLocked: locked[row.ID]},
		}
	}
	return products, total, nil
}

// ProductsMatching возвращает все продукты компании по алфавиту, название которых начинается
// с namePrefix без учёта регистра, а категория равна category. Пустое значение условия не задаёт.
func (s *Service) ProductsMatching(ctx context.Context, companyID uuid.UUID, namePrefix, category string) ([]Product, error) {
	rows, err := db.New(s.pool).SearchProducts(ctx, db.SearchProductsParams{
		CompanyID: companyID, NamePrefix: nameKey(strings.TrimSpace(namePrefix)), Category: category,
	})
	if err != nil {
		return nil, fmt.Errorf("catalog: search products: %w", err)
	}
	products := make([]Product, len(rows))
	for i, row := range rows {
		products[i] = productFromRow(row.ID, row.CompanyID, row.Name, row.Unit, row.Category, row.ShelfLifeDays, row.Archived)
	}
	return products, nil
}

// Dish возвращает блюдо с рецептурой или ErrNotFound.
func (s *Service) Dish(ctx context.Context, id uuid.UUID) (Dish, error) {
	return s.DishIn(ctx, s.pool, id)
}

// Querier - соединение, через которое читается каталог: пул или транзакция команды.
type Querier = db.DBTX

// DishIn читает блюдо через переданное соединение. Команде другого модуля оно нужно,
// чтобы прочитать рецептуру в своей транзакции, уже под блокировками.
func (s *Service) DishIn(ctx context.Context, q Querier, id uuid.UUID) (Dish, error) {
	return readDish(ctx, db.New(q), id)
}

// Dishes возвращает страницу блюд компании с рецептурами под фильтры по алфавиту и их общее число.
func (s *Service) Dishes(ctx context.Context, companyID uuid.UUID, filter ListFilter, page Page) ([]Dish, int64, error) {
	return s.dishes(ctx, companyID, filter, nil, page)
}

// DishesOnPoint возвращает страницу блюд компании, которые продаются на её точке: без архивных
// и без выключенных на этой точке. По ним строятся меню, стоп-лист, рекомендации и прогноз точки.
func (s *Service) DishesOnPoint(ctx context.Context, companyID, pointID uuid.UUID, page Page) ([]Dish, int64, error) {
	return s.dishes(ctx, companyID, ListFilter{}, &pointID, page)
}

func (s *Service) dishes(ctx context.Context, companyID uuid.UUID, filter ListFilter, soldOnPoint *uuid.UUID, page Page) ([]Dish, int64, error) {
	q := db.New(s.pool)
	part := namePart(filter.Query)
	rows, err := q.ListDishes(ctx, db.ListDishesParams{
		CompanyID: companyID, Archived: filter.Archived, NamePart: part, MissingRecipe: filter.MissingRecipe,
		SoldOnPoint: soldOnPoint, PageLimit: page.Limit, PageOffset: page.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list dishes: %w", err)
	}
	total, err := q.CountDishes(ctx, db.CountDishesParams{
		CompanyID: companyID, Archived: filter.Archived, NamePart: part, MissingRecipe: filter.MissingRecipe,
		SoldOnPoint: soldOnPoint,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: count dishes: %w", err)
	}

	// рецептуры читаются одним запросом, только для блюд страницы
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	ingredients, err := q.ListIngredientsOfDishes(ctx, ids)
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list ingredients: %w", err)
	}
	byDish := make(map[uuid.UUID][]Ingredient, len(rows))
	for _, ing := range ingredients {
		byDish[ing.DishID] = append(byDish[ing.DishID], Ingredient{
			ProductID: ing.ProductID, Quantity: ing.Quantity, ProductName: ing.Name, Unit: Unit(ing.Unit),
			ProductArchived: ing.Archived,
		})
	}

	dishes := make([]Dish, len(rows))
	for i, row := range rows {
		dishes[i] = Dish{
			ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, Ingredients: orEmpty(byDish[row.ID]), Archived: row.Archived,
		}
	}
	return dishes, total, nil
}

// SetDishOnPoint выключает или включает блюдо компании на её точке. Точку и её компанию
// определяет вызывающий; блюдо другой компании - ErrNotFound. Повтор действующего состояния
// события не пишет.
func (s *Service) SetDishOnPoint(ctx context.Context, actor eventstore.Actor, key string, companyID, pointID, dishID uuid.UUID, enabled bool) (PointDish, error) {
	stream := MenuStream(pointID)
	spec := command.Spec{
		Name: "SetDishOnPoint", Actor: actor, IdempotencyKey: key, Locks: []string{stream},
		Request: struct {
			PointID, DishID uuid.UUID
			Enabled         bool
		}{pointID, dishID, enabled},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (PointDish, error) {
		q := db.New(tx)
		dish, err := q.GetDish(ctx, dishID)
		if err != nil {
			return PointDish{}, notFound(err, "dish", dishID)
		}
		if dish.CompanyID != companyID {
			return PointDish{}, fmt.Errorf("dish %s: %w", dishID, ErrNotFound)
		}
		disabled, err := q.DishDisabled(ctx, db.DishDisabledParams{PointID: pointID, DishID: dishID})
		if err != nil {
			return PointDish{}, fmt.Errorf("catalog: read dish on point: %w", err)
		}
		if disabled == enabled {
			version, err := eventstore.StreamVersion(ctx, tx, stream)
			if err != nil {
				return PointDish{}, err
			}
			event := DishAvailabilityChanged{PointID: pointID, DishID: dishID, Enabled: enabled}
			if _, err := tx.Append(ctx, stream, version, event); err != nil {
				return PointDish{}, err
			}
			if err := applyDishAvailabilityChanged(ctx, q, event); err != nil {
				return PointDish{}, err
			}
		}
		return PointDish{DishID: dishID, Name: dish.Name, Enabled: enabled}, nil
	})
}

// DisabledOnPoint возвращает блюда из ids, которые на точке не продаются: выключенные на ней
// или в архиве. Читает через переданное соединение: заказу это нужно в своей транзакции.
func (s *Service) DisabledOnPoint(ctx context.Context, q Querier, pointID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	disabled, err := db.New(q).UnsoldDishes(ctx, db.UnsoldDishesParams{PointID: pointID, DishIds: ids})
	if err != nil {
		return nil, fmt.Errorf("catalog: list disabled dishes: %w", err)
	}
	return disabled, nil
}

// PointDishes возвращает страницу неархивных блюд компании по алфавиту с признаком, продаётся ли
// каждое на точке, и их общее число.
func (s *Service) PointDishes(ctx context.Context, companyID, pointID uuid.UUID, page Page) ([]PointDish, int64, error) {
	q := db.New(s.pool)
	rows, err := q.ListDishes(ctx, db.ListDishesParams{CompanyID: companyID, PageLimit: page.Limit, PageOffset: page.Offset})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list dishes: %w", err)
	}
	total, err := q.CountDishes(ctx, db.CountDishesParams{CompanyID: companyID})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: count dishes: %w", err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	disabled, err := q.ListDisabledDishes(ctx, db.ListDisabledDishesParams{PointID: pointID, DishIds: ids})
	if err != nil {
		return nil, 0, fmt.Errorf("catalog: list disabled dishes: %w", err)
	}
	dishes := make([]PointDish, len(rows))
	for i, row := range rows {
		dishes[i] = PointDish{DishID: row.ID, Name: row.Name, Enabled: !slices.Contains(disabled, row.ID)}
	}
	return dishes, total, nil
}

// Таблицы каталога меняются только функциями apply*: их содержимое - функция от событий,
// поэтому перестроение из Event Store сведётся к вызову этих же функций.

func applyProductCreated(ctx context.Context, q *db.Queries, e ProductCreated) error {
	var shelfLife *int32
	if e.ShelfLifeDays != nil {
		days := int32(*e.ShelfLifeDays)
		shelfLife = &days
	}
	err := q.InsertProduct(ctx, db.InsertProductParams{
		ID: e.ProductID, CompanyID: e.CompanyID, Name: e.Name, NameKey: nameKey(e.Name),
		Unit: string(e.Unit), Category: e.Category, ShelfLifeDays: shelfLife,
	})
	return nameTaken(err, "catalog_products_name_key", "product", e.Name)
}

func applyDishCreated(ctx context.Context, q *db.Queries, e DishCreated) error {
	err := q.InsertDish(ctx, db.InsertDishParams{ID: e.DishID, CompanyID: e.CompanyID, Name: e.Name, NameKey: nameKey(e.Name)})
	return nameTaken(err, "catalog_dishes_name_key", "dish", e.Name)
}

func applyProductChanged(ctx context.Context, q *db.Queries, e ProductChanged) error {
	var shelfLife *int32
	if e.ShelfLifeDays != nil {
		days := int32(*e.ShelfLifeDays)
		shelfLife = &days
	}
	err := q.UpdateProduct(ctx, db.UpdateProductParams{
		ID: e.ProductID, Name: e.Name, NameKey: nameKey(e.Name), Unit: string(e.Unit), Category: e.Category,
		ShelfLifeDays: shelfLife,
	})
	return nameTaken(err, "catalog_products_name_key", "product", e.Name)
}

func applyProductArchived(ctx context.Context, q *db.Queries, productID uuid.UUID, archived bool) error {
	if err := q.SetProductArchived(ctx, db.SetProductArchivedParams{ID: productID, Archived: archived}); err != nil {
		return fmt.Errorf("catalog: save product archive: %w", err)
	}
	return nil
}

func applyDishRenamed(ctx context.Context, q *db.Queries, e DishRenamed) error {
	err := q.RenameDish(ctx, db.RenameDishParams{ID: e.DishID, Name: e.Name, NameKey: nameKey(e.Name)})
	return nameTaken(err, "catalog_dishes_name_key", "dish", e.Name)
}

func applyDishArchived(ctx context.Context, q *db.Queries, dishID uuid.UUID, archived bool) error {
	if err := q.SetDishArchived(ctx, db.SetDishArchivedParams{ID: dishID, Archived: archived}); err != nil {
		return fmt.Errorf("catalog: save dish archive: %w", err)
	}
	return nil
}

func applyRecipeChanged(ctx context.Context, q *db.Queries, e RecipeChanged) error {
	if err := q.DeleteIngredients(ctx, e.DishID); err != nil {
		return fmt.Errorf("catalog: replace recipe: %w", err)
	}
	for _, ing := range e.Ingredients {
		err := q.InsertIngredient(ctx, db.InsertIngredientParams{
			DishID: e.DishID, ProductID: ing.ProductID, Quantity: ing.Quantity,
		})
		if err != nil {
			return fmt.Errorf("catalog: replace recipe: %w", err)
		}
	}
	return nil
}

func applyDishAvailabilityChanged(ctx context.Context, q *db.Queries, e DishAvailabilityChanged) error {
	var err error
	if e.Enabled {
		err = q.EnableDish(ctx, db.EnableDishParams{PointID: e.PointID, DishID: e.DishID})
	} else {
		err = q.DisableDish(ctx, db.DisableDishParams{PointID: e.PointID, DishID: e.DishID})
	}
	if err != nil {
		return fmt.Errorf("catalog: save dish on point: %w", err)
	}
	return nil
}

// requireProducts проверяет, что все продукты рецептуры есть в каталоге компании блюда и не в архиве.
func requireProducts(ctx context.Context, q *db.Queries, companyID uuid.UUID, ingredients []Ingredient) error {
	ids := make([]uuid.UUID, len(ingredients))
	for i, ing := range ingredients {
		ids[i] = ing.ProductID
	}
	existing, err := q.ExistingProducts(ctx, db.ExistingProductsParams{CompanyID: companyID, Ids: ids})
	if err != nil {
		return fmt.Errorf("catalog: check products: %w", err)
	}
	known := make(map[uuid.UUID]db.ExistingProductsRow, len(existing))
	for _, row := range existing {
		known[row.ID] = row
	}
	for _, id := range ids {
		row, ok := known[id]
		switch {
		case !ok:
			return &UnknownProductError{ProductID: id}
		case row.Archived:
			return &ArchivedProductError{ProductID: id, Name: row.Name}
		}
	}
	return nil
}

// productNameFree проверяет, что название не занято другим продуктом компании. self - сам
// изменяемый продукт: своё название не мешает.
func productNameFree(ctx context.Context, q *db.Queries, companyID uuid.UUID, name string, self uuid.UUID) error {
	row, err := q.ProductWithName(ctx, db.ProductWithNameParams{CompanyID: companyID, NameKey: nameKey(name)})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("catalog: check product name: %w", err)
	case row.ID == self:
		return nil
	}
	return &NameTakenError{What: "product", Name: name, Archived: row.Archived}
}

// dishNameFree - то же для блюд.
func dishNameFree(ctx context.Context, q *db.Queries, companyID uuid.UUID, name string, self uuid.UUID) error {
	row, err := q.DishWithName(ctx, db.DishWithNameParams{CompanyID: companyID, NameKey: nameKey(name)})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("catalog: check dish name: %w", err)
	case row.ID == self:
		return nil
	}
	return &NameTakenError{What: "dish", Name: name, Archived: row.Archived}
}

func readProduct(ctx context.Context, q *db.Queries, id uuid.UUID) (Product, error) {
	row, err := q.GetProduct(ctx, id)
	if err != nil {
		return Product{}, notFound(err, "product", id)
	}
	return productFromRow(row.ID, row.CompanyID, row.Name, row.Unit, row.Category, row.ShelfLifeDays, row.Archived), nil
}

func readDish(ctx context.Context, q *db.Queries, id uuid.UUID) (Dish, error) {
	row, err := q.GetDish(ctx, id)
	if err != nil {
		return Dish{}, notFound(err, "dish", id)
	}
	rows, err := q.ListIngredients(ctx, id)
	if err != nil {
		return Dish{}, fmt.Errorf("catalog: read recipe: %w", err)
	}
	ingredients := make([]Ingredient, len(rows))
	for i, r := range rows {
		ingredients[i] = Ingredient{
			ProductID: r.ProductID, Quantity: r.Quantity, ProductName: r.Name, Unit: Unit(r.Unit), ProductArchived: r.Archived,
		}
	}
	return Dish{ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, Ingredients: ingredients, Archived: row.Archived}, nil
}

func productFromRow(id, companyID uuid.UUID, name, unit, category string, shelfLife *int32, archived bool) Product {
	p := Product{ID: id, CompanyID: companyID, Name: name, Unit: Unit(unit), Category: category, Archived: archived}
	if shelfLife != nil {
		days := int(*shelfLife)
		p.ShelfLifeDays = &days
	}
	return p
}

// orEmpty заменяет nil пустым срезом: блюдо без рецептуры одинаково выглядит
// в ответе команды, при чтении и в сохранённом результате повтора.
func orEmpty(ingredients []Ingredient) []Ingredient {
	if ingredients == nil {
		return []Ingredient{}
	}
	return ingredients
}

func notFound(err error, what string, id uuid.UUID) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s %s: %w", what, id, ErrNotFound)
	}
	return fmt.Errorf("catalog: read %s: %w", what, err)
}

// nameTaken превращает нарушение уникальности названия в NameTakenError.
// Срабатывает при гонке двух одновременных команд: обе прошли проверку названия.
func nameTaken(err error, constraint, what, name string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == constraint {
		return &NameTakenError{What: what, Name: name}
	}
	return fmt.Errorf("catalog: save %s: %w", what, err)
}
