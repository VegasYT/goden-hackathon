package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
)

// HTTP реализует операции каталога из api/openapi.yaml. Обработчик - тонкая обёртка:
// разбирает запрос, вызывает сервис, собирает ответ. Правил в нём нет.
//
// Право вызывающего в компании из адреса проверил слой API. Обработчик отвечает за второе
// условие: объект по идентификатору принадлежит этой компании, иначе его как будто нет.
type HTTP struct {
	svc *Service
	// points сообщает компанию точки для операций меню точки
	points *organization.Service
}

func NewHTTP(svc *Service, points *organization.Service) *HTTP {
	return &HTTP{svc: svc, points: points}
}

// CreateProduct реализует POST /companies/{companyId}/products.
func (h *HTTP) CreateProduct(ctx context.Context, req *oas.NewProduct, params oas.CreateProductParams) (*oas.Product, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("catalog: request has no actor")
	}
	if _, err := h.points.WorkingCompany(ctx, params.CompanyId); err != nil {
		return nil, err
	}
	in := NewProduct{Name: req.Name, Unit: Unit(req.Unit), Category: req.Category.Or("")}
	if days, set := req.ShelfLifeDays.Get(); set {
		shelfLife := int(days)
		in.ShelfLifeDays = &shelfLife
	}

	product, err := h.svc.CreateProduct(ctx, actor, params.IdempotencyKey.Or(""), params.CompanyId, in)
	if err != nil {
		return nil, err
	}
	// новый продукт ещё нигде не используется
	return catalogProduct(product, ProductUse{}), nil
}

// GetProduct реализует GET /companies/{companyId}/products/{productId}.
func (h *HTTP) GetProduct(ctx context.Context, params oas.GetProductParams) (*oas.Product, error) {
	product, err := h.svc.ProductOf(ctx, params.CompanyId, params.ProductId)
	if err != nil {
		return nil, err
	}
	return h.productWithUse(ctx, product)
}

// productWithUse - продукт каталога с тем, где он используется, и блюдами с ним.
func (h *HTTP) productWithUse(ctx context.Context, product Product) (*oas.Product, error) {
	use, dishes, err := h.svc.ProductUse(ctx, product.ID)
	if err != nil {
		return nil, err
	}
	out := catalogProduct(product, use)
	out.UsedIn = make([]oas.DishRef, len(dishes))
	for i, dish := range dishes {
		out.UsedIn[i] = oas.DishRef{ID: dish.ID, Name: dish.Name}
	}
	return out, nil
}

// ListProducts реализует GET /companies/{companyId}/products.
func (h *HTTP) ListProducts(ctx context.Context, params oas.ListProductsParams) (*oas.ProductPage, error) {
	// значения по умолчанию подставляет сгенерированный разбор запроса
	filter := ListFilter{
		Query: params.Q.Or(""), Category: params.Category.Or(""),
		Archived:   params.Status.Or(oas.CatalogStatusActive) == oas.CatalogStatusArchived,
		ByCategory: params.Sort.Or(oas.ListProductsSortName) == oas.ListProductsSortCategory,
	}
	products, total, err := h.svc.ListProducts(ctx, params.CompanyId, filter, Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.ProductPage{Items: make([]oas.Product, len(products)), Total: total}
	for i, product := range products {
		page.Items[i] = *catalogProduct(product.Product, product.Use)
	}
	return page, nil
}

// ChangeProduct реализует PATCH /companies/{companyId}/products/{productId}.
func (h *HTTP) ChangeProduct(ctx context.Context, req *oas.ProductChange, params oas.ChangeProductParams) (*oas.Product, error) {
	actor, err := h.writable(ctx, params.CompanyId)
	if err != nil {
		return nil, err
	}
	// компания продукта не меняется, поэтому сверка до команды не устаревает
	if _, err := h.svc.ProductOf(ctx, params.CompanyId, params.ProductId); err != nil {
		return nil, err
	}
	var change ProductChange
	if name, set := req.Name.Get(); set {
		change.Name = &name
	}
	if unit, set := req.Unit.Get(); set {
		u := Unit(unit)
		change.Unit = &u
	}
	if category, set := req.Category.Get(); set {
		change.Category = &category
	}
	switch days, set := req.ShelfLifeDays.Get(); {
	case req.ShelfLifeDays.IsNull():
		change.ClearShelfLife = true
	case set:
		shelfLife := int(days)
		change.ShelfLifeDays = &shelfLife
	}

	product, err := h.svc.ChangeProduct(ctx, actor, params.IdempotencyKey.Or(""), params.ProductId, change)
	if err != nil {
		return nil, err
	}
	return h.productWithUse(ctx, product)
}

// ArchiveProduct реализует POST /companies/{companyId}/products/{productId}/archive.
func (h *HTTP) ArchiveProduct(ctx context.Context, params oas.ArchiveProductParams) (*oas.Product, error) {
	return h.setProductArchived(ctx, params.CompanyId, params.ProductId, params.IdempotencyKey.Or(""), h.svc.ArchiveProduct)
}

// RestoreProduct реализует POST /companies/{companyId}/products/{productId}/restore.
func (h *HTTP) RestoreProduct(ctx context.Context, params oas.RestoreProductParams) (*oas.Product, error) {
	return h.setProductArchived(ctx, params.CompanyId, params.ProductId, params.IdempotencyKey.Or(""), h.svc.RestoreProduct)
}

func (h *HTTP) setProductArchived(
	ctx context.Context, companyID, productID uuid.UUID, key string,
	command func(context.Context, eventstore.Actor, string, uuid.UUID) (Product, error),
) (*oas.Product, error) {
	actor, err := h.writable(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.ProductOf(ctx, companyID, productID); err != nil {
		return nil, err
	}
	product, err := command(ctx, actor, key, productID)
	if err != nil {
		return nil, err
	}
	return h.productWithUse(ctx, product)
}

// ListProductCategories реализует GET /companies/{companyId}/product-categories.
func (h *HTTP) ListProductCategories(ctx context.Context, params oas.ListProductCategoriesParams) (*oas.ProductCategories, error) {
	categories, err := h.svc.ProductCategories(ctx, params.CompanyId)
	if err != nil {
		return nil, err
	}
	out := &oas.ProductCategories{Items: make([]oas.ProductCategory, len(categories))}
	for i, category := range categories {
		out.Items[i] = oas.ProductCategory{Name: category.Name, Products: category.Products}
	}
	return out, nil
}

// writable возвращает автора запроса на изменение каталога, если компания работает.
func (h *HTTP) writable(ctx context.Context, companyID uuid.UUID) (eventstore.Actor, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return eventstore.Actor{}, errors.New("catalog: request has no actor")
	}
	if _, err := h.points.WorkingCompany(ctx, companyID); err != nil {
		return eventstore.Actor{}, err
	}
	return actor, nil
}

// catalogProduct - продукт в ответе каталога: с числом блюд и признаком закреплённой единицы.
func catalogProduct(p Product, use ProductUse) *oas.Product {
	out := ProductResponse(p)
	out.Dishes = oas.NewOptInt64(use.Dishes)
	out.UnitLocked = oas.NewOptBool(use.UnitLocked)
	return out
}

// ProductResponse переводит продукт в схему контракта. Нужен и другим модулям:
// остатки по складу отдают продукт целиком.
func ProductResponse(p Product) *oas.Product {
	out := &oas.Product{
		ID: p.ID, CompanyID: p.CompanyID, Name: p.Name, Unit: oas.Unit(p.Unit), Category: p.Category, Archived: p.Archived,
	}
	if p.ShelfLifeDays != nil {
		out.ShelfLifeDays = oas.NewOptInt32(int32(*p.ShelfLifeDays))
	}
	return out
}

// CreateDish реализует POST /companies/{companyId}/dishes.
func (h *HTTP) CreateDish(ctx context.Context, req *oas.NewDish, params oas.CreateDishParams) (*oas.Dish, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("catalog: request has no actor")
	}
	if _, err := h.points.WorkingCompany(ctx, params.CompanyId); err != nil {
		return nil, err
	}
	dish, err := h.svc.CreateDish(ctx, actor, params.IdempotencyKey.Or(""), params.CompanyId, req.Name, ingredientsOf(req.Ingredients)...)
	if err != nil {
		return nil, err
	}
	return dishResponse(dish), nil
}

func ingredientsOf(in []oas.Ingredient) []Ingredient {
	ingredients := make([]Ingredient, len(in))
	for i, ingredient := range in {
		ingredients[i] = Ingredient{ProductID: ingredient.ProductID, Quantity: ingredient.Quantity}
	}
	return ingredients
}

// RenameDish реализует PATCH /companies/{companyId}/dishes/{dishId}.
func (h *HTTP) RenameDish(ctx context.Context, req *oas.DishChange, params oas.RenameDishParams) (*oas.Dish, error) {
	actor, err := h.writable(ctx, params.CompanyId)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.DishOf(ctx, params.CompanyId, params.DishId); err != nil {
		return nil, err
	}
	dish, err := h.svc.RenameDish(ctx, actor, params.IdempotencyKey.Or(""), params.DishId, req.Name)
	if err != nil {
		return nil, err
	}
	return dishResponse(dish), nil
}

// ArchiveDish реализует POST /companies/{companyId}/dishes/{dishId}/archive.
func (h *HTTP) ArchiveDish(ctx context.Context, params oas.ArchiveDishParams) (*oas.Dish, error) {
	return h.setDishArchived(ctx, params.CompanyId, params.DishId, params.IdempotencyKey.Or(""), h.svc.ArchiveDish)
}

// RestoreDish реализует POST /companies/{companyId}/dishes/{dishId}/restore.
func (h *HTTP) RestoreDish(ctx context.Context, params oas.RestoreDishParams) (*oas.Dish, error) {
	return h.setDishArchived(ctx, params.CompanyId, params.DishId, params.IdempotencyKey.Or(""), h.svc.RestoreDish)
}

func (h *HTTP) setDishArchived(
	ctx context.Context, companyID, dishID uuid.UUID, key string,
	command func(context.Context, eventstore.Actor, string, uuid.UUID) (Dish, error),
) (*oas.Dish, error) {
	actor, err := h.writable(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.DishOf(ctx, companyID, dishID); err != nil {
		return nil, err
	}
	dish, err := command(ctx, actor, key, dishID)
	if err != nil {
		return nil, err
	}
	return dishResponse(dish), nil
}

// GetDish реализует GET /companies/{companyId}/dishes/{dishId}.
func (h *HTTP) GetDish(ctx context.Context, params oas.GetDishParams) (*oas.Dish, error) {
	dish, err := h.svc.DishOf(ctx, params.CompanyId, params.DishId)
	if err != nil {
		return nil, err
	}
	return dishResponse(dish), nil
}

// ListDishes реализует GET /companies/{companyId}/dishes.
func (h *HTTP) ListDishes(ctx context.Context, params oas.ListDishesParams) (*oas.DishPage, error) {
	filter := ListFilter{
		Query:         params.Q.Or(""),
		Archived:      params.Status.Or(oas.CatalogStatusActive) == oas.CatalogStatusArchived,
		MissingRecipe: params.Recipe.IsSet(),
	}
	dishes, total, err := h.svc.Dishes(ctx, params.CompanyId, filter, Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.DishPage{Items: make([]oas.Dish, len(dishes)), Total: total}
	for i, dish := range dishes {
		page.Items[i] = *dishResponse(dish)
	}
	return page, nil
}

// ChangeRecipe реализует PUT /companies/{companyId}/dishes/{dishId}/recipe.
func (h *HTTP) ChangeRecipe(ctx context.Context, req *oas.Recipe, params oas.ChangeRecipeParams) (*oas.Dish, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("catalog: request has no actor")
	}
	if _, err := h.points.WorkingCompany(ctx, params.CompanyId); err != nil {
		return nil, err
	}
	// компания блюда не меняется, поэтому сверка до команды не устаревает
	if _, err := h.svc.DishOf(ctx, params.CompanyId, params.DishId); err != nil {
		return nil, err
	}
	dish, err := h.svc.ChangeRecipe(ctx, actor, params.IdempotencyKey.Or(""), params.DishId, ingredientsOf(req.Ingredients))
	if err != nil {
		return nil, err
	}
	return dishResponse(dish), nil
}

func dishResponse(d Dish) *oas.Dish {
	out := &oas.Dish{
		ID: d.ID, CompanyID: d.CompanyID, Name: d.Name, Archived: d.Archived,
		Ingredients: make([]oas.DishIngredient, len(d.Ingredients)),
	}
	for i, ingredient := range d.Ingredients {
		out.Ingredients[i] = oas.DishIngredient{
			ProductID: ingredient.ProductID, Quantity: ingredient.Quantity, ProductName: ingredient.ProductName,
			Unit: oas.Unit(ingredient.Unit), ProductArchived: ingredient.ProductArchived,
		}
	}
	return out
}

// ListPointDishes реализует GET /points/{pointId}/dishes.
func (h *HTTP) ListPointDishes(ctx context.Context, params oas.ListPointDishesParams) (*oas.PointDishPage, error) {
	point, err := h.points.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	dishes, total, err := h.svc.PointDishes(ctx, point.CompanyID, point.ID, Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.PointDishPage{Items: make([]oas.PointDish, len(dishes)), Total: total}
	for i, dish := range dishes {
		page.Items[i] = oas.PointDish(dish)
	}
	return page, nil
}

// SetDishOnPoint реализует PUT /points/{pointId}/dishes/{dishId}.
func (h *HTTP) SetDishOnPoint(ctx context.Context, req *oas.DishAvailabilityOnPoint, params oas.SetDishOnPointParams) (*oas.PointDish, error) {
	actor, ok := httpx.Actor(ctx)
	if !ok {
		return nil, errors.New("catalog: request has no actor")
	}
	point, err := h.points.WorkingPoint(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	dish, err := h.svc.SetDishOnPoint(ctx, actor, params.IdempotencyKey.Or(""), point.CompanyID, point.ID, params.DishId, req.Enabled)
	if err != nil {
		return nil, err
	}
	out := oas.PointDish(dish)
	return &out, nil
}
