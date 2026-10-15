-- +goose Up

-- Синхронные таблицы каталога: обновляются в одной транзакции с событиями.
-- Источник истины - Event Store, таблицы принадлежат модулю catalog.
CREATE TABLE catalog_products (
    id              uuid PRIMARY KEY,
    name            text NOT NULL,
    -- название в нижнем регистре без пробелов по краям: по нему проверяется уникальность.
    -- Считается в приложении, чтобы результат не зависел от локали базы.
    name_key        text NOT NULL,
    unit            text NOT NULL CHECK (unit IN ('g', 'ml', 'pcs')),
    category        text NOT NULL,
    shelf_life_days integer CHECK (shelf_life_days > 0),
    CONSTRAINT catalog_products_name_key UNIQUE (name_key)
);

CREATE TABLE catalog_dishes (
    id       uuid PRIMARY KEY,
    name     text NOT NULL,
    name_key text NOT NULL,
    CONSTRAINT catalog_dishes_name_key UNIQUE (name_key)
);

CREATE TABLE catalog_recipe_ingredients (
    dish_id    uuid   NOT NULL REFERENCES catalog_dishes (id),
    product_id uuid   NOT NULL REFERENCES catalog_products (id),
    -- количество на одну порцию в единице измерения продукта
    quantity   bigint NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (dish_id, product_id)
);

-- +goose Down

DROP TABLE catalog_recipe_ingredients;
DROP TABLE catalog_dishes;
DROP TABLE catalog_products;
