-- name: InsertProduct :exec
INSERT INTO catalog_products (id, company_id, name, name_key, unit, category, shelf_life_days)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetProduct :one
SELECT id, company_id, name, unit, category, shelf_life_days, archived
FROM catalog_products
WHERE id = $1;

-- Продукты компании под фильтры списка. name_part - часть названия в нижнем регистре
-- с экранированными знаками шаблона. dishes - неархивные блюда с продуктом в рецептуре,
-- in_recipe - продукт есть в рецептуре любого блюда, в том числе архивного.
-- name: ListProducts :many
SELECT p.id, p.company_id, p.name, p.unit, p.category, p.shelf_life_days, p.archived,
       (SELECT count(*)
        FROM catalog_recipe_ingredients i
        JOIN catalog_dishes d ON d.id = i.dish_id
        WHERE i.product_id = p.id AND NOT d.archived)::bigint AS dishes,
       EXISTS (SELECT 1 FROM catalog_recipe_ingredients i WHERE i.product_id = p.id) AS in_recipe
FROM catalog_products p
WHERE p.company_id = sqlc.arg(company_id)
  AND p.archived = sqlc.arg(archived)
  AND (sqlc.arg(name_part)::text = '' OR p.name_key LIKE '%' || sqlc.arg(name_part)::text || '%')
  AND (sqlc.arg(category)::text = '' OR p.category = sqlc.arg(category)::text)
-- по категории: продукты без категории в конце, внутри категории по названию
ORDER BY CASE WHEN sqlc.arg(by_category)::boolean THEN p.category = '' END,
         CASE WHEN sqlc.arg(by_category)::boolean THEN p.category END,
         p.name_key, p.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ProductRecipeUse :one
-- Сколько неархивных блюд с продуктом в рецептуре и есть ли он в рецептуре любого блюда.
SELECT (SELECT count(*)
        FROM catalog_recipe_ingredients i
        JOIN catalog_dishes d ON d.id = i.dish_id
        WHERE i.product_id = sqlc.arg(product_id) AND NOT d.archived)::bigint AS dishes,
       EXISTS (SELECT 1 FROM catalog_recipe_ingredients i WHERE i.product_id = sqlc.arg(product_id)) AS in_recipe;

-- name: DishesWithProduct :many
-- Неархивные блюда с продуктом в рецептуре, по названию.
SELECT d.id, d.name
FROM catalog_recipe_ingredients i
JOIN catalog_dishes d ON d.id = i.dish_id
WHERE i.product_id = $1 AND NOT d.archived
ORDER BY d.name_key, d.id;

-- name: ProductCategories :many
-- Категории компании с числом неархивных продуктов; пустая категория не входит.
SELECT category, count(*)::bigint AS products
FROM catalog_products
WHERE company_id = $1 AND NOT archived AND category <> ''
GROUP BY category
ORDER BY category;

-- name: UpdateProduct :exec
UPDATE catalog_products
SET name = $2, name_key = $3, unit = $4, category = $5, shelf_life_days = $6
WHERE id = $1;

-- name: SetProductArchived :exec
UPDATE catalog_products SET archived = $2 WHERE id = $1;

-- name: SearchProducts :many
-- Все продукты компании под условия, по названию, в том числе архивные. Знаки шаблона
-- в начале названия не действуют.
SELECT id, company_id, name, unit, category, shelf_life_days, archived
FROM catalog_products
WHERE company_id = sqlc.arg(company_id)
  AND starts_with(name_key, sqlc.arg(name_prefix)::text)
  AND (sqlc.arg(category)::text = '' OR category = sqlc.arg(category)::text)
ORDER BY name_key, id;

-- name: CountProducts :one
SELECT count(*)
FROM catalog_products p
WHERE p.company_id = sqlc.arg(company_id)
  AND p.archived = sqlc.arg(archived)
  AND (sqlc.arg(name_part)::text = '' OR p.name_key LIKE '%' || sqlc.arg(name_part)::text || '%')
  AND (sqlc.arg(category)::text = '' OR p.category = sqlc.arg(category)::text);

-- name: ProductWithName :one
-- Продукт компании, который занимает название.
SELECT id, archived FROM catalog_products WHERE company_id = $1 AND name_key = $2;

-- продукты компании среди перечисленных
-- name: ExistingProducts :many
SELECT id, name, archived
FROM catalog_products
WHERE company_id = sqlc.arg(company_id) AND id = ANY (sqlc.arg(ids)::uuid[]);

-- name: InsertDish :exec
INSERT INTO catalog_dishes (id, company_id, name, name_key)
VALUES ($1, $2, $3, $4);

-- name: GetDish :one
SELECT id, company_id, name, archived
FROM catalog_dishes
WHERE id = $1;

-- Блюда компании под фильтры списка. С точкой - только те, что на ней продаются: без выключенных.
-- name: ListDishes :many
SELECT c.id, c.company_id, c.name, c.archived
FROM catalog_dishes c
WHERE c.company_id = sqlc.arg(company_id)
  AND c.archived = sqlc.arg(archived)
  AND (sqlc.arg(name_part)::text = '' OR c.name_key LIKE '%' || sqlc.arg(name_part)::text || '%')
  AND (NOT sqlc.arg(missing_recipe)::boolean OR NOT EXISTS (
      SELECT 1 FROM catalog_recipe_ingredients i WHERE i.dish_id = c.id))
  AND (sqlc.narg(sold_on_point)::uuid IS NULL OR NOT EXISTS (
      SELECT 1 FROM catalog_disabled_dishes d WHERE d.point_id = sqlc.narg(sold_on_point)::uuid AND d.dish_id = c.id))
ORDER BY c.name_key, c.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountDishes :one
SELECT count(*)
FROM catalog_dishes c
WHERE c.company_id = sqlc.arg(company_id)
  AND c.archived = sqlc.arg(archived)
  AND (sqlc.arg(name_part)::text = '' OR c.name_key LIKE '%' || sqlc.arg(name_part)::text || '%')
  AND (NOT sqlc.arg(missing_recipe)::boolean OR NOT EXISTS (
      SELECT 1 FROM catalog_recipe_ingredients i WHERE i.dish_id = c.id))
  AND (sqlc.narg(sold_on_point)::uuid IS NULL OR NOT EXISTS (
      SELECT 1 FROM catalog_disabled_dishes d WHERE d.point_id = sqlc.narg(sold_on_point)::uuid AND d.dish_id = c.id));

-- name: DishWithName :one
-- Блюдо компании, которое занимает название.
SELECT id, archived FROM catalog_dishes WHERE company_id = $1 AND name_key = $2;

-- name: RenameDish :exec
UPDATE catalog_dishes SET name = $2, name_key = $3 WHERE id = $1;

-- name: SetDishArchived :exec
UPDATE catalog_dishes SET archived = $2 WHERE id = $1;

-- name: ArchivedProductsOfDish :many
-- Архивные продукты рецептуры блюда, по названию.
SELECT p.name
FROM catalog_recipe_ingredients i
JOIN catalog_products p ON p.id = i.product_id
WHERE i.dish_id = $1 AND p.archived
ORDER BY p.name_key;

-- name: DeleteIngredients :exec
DELETE FROM catalog_recipe_ingredients
WHERE dish_id = $1;

-- name: InsertIngredient :exec
INSERT INTO catalog_recipe_ingredients (dish_id, product_id, quantity)
VALUES ($1, $2, $3);

-- name: ListIngredients :many
SELECT i.dish_id, i.product_id, i.quantity, p.name, p.unit, p.archived
FROM catalog_recipe_ingredients i
JOIN catalog_products p ON p.id = i.product_id
WHERE i.dish_id = $1
ORDER BY p.name_key;

-- name: ListIngredientsOfDishes :many
SELECT i.dish_id, i.product_id, i.quantity, p.name, p.unit, p.archived
FROM catalog_recipe_ingredients i
JOIN catalog_products p ON p.id = i.product_id
WHERE i.dish_id = ANY (sqlc.arg(dish_ids)::uuid[])
ORDER BY i.dish_id, p.name_key;

-- name: GetProductsByIDs :many
SELECT id, company_id, name, unit, category, shelf_life_days, archived
FROM catalog_products
WHERE id = ANY (sqlc.arg(ids)::uuid[]);

-- name: DisableDish :exec
INSERT INTO catalog_disabled_dishes (point_id, dish_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: EnableDish :exec
DELETE FROM catalog_disabled_dishes
WHERE point_id = $1 AND dish_id = $2;

-- name: DishDisabled :one
SELECT EXISTS (SELECT 1 FROM catalog_disabled_dishes WHERE point_id = $1 AND dish_id = $2);

-- name: ListDisabledDishes :many
SELECT dish_id
FROM catalog_disabled_dishes
WHERE point_id = sqlc.arg(point_id) AND dish_id = ANY (sqlc.arg(dish_ids)::uuid[]);

-- name: UnsoldDishes :many
-- Блюда из перечисленных, которые на точке не продаются: выключенные на ней или в архиве.
SELECT c.id
FROM catalog_dishes c
WHERE c.id = ANY (sqlc.arg(dish_ids)::uuid[])
  AND (c.archived OR EXISTS (
      SELECT 1 FROM catalog_disabled_dishes d WHERE d.point_id = sqlc.arg(point_id) AND d.dish_id = c.id));
