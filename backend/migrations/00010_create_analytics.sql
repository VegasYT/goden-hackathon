-- +goose Up

-- Факты аналитики. event_id защищает проекцию от повторного применения события.
CREATE TABLE analytics_applied_events (
    event_id uuid PRIMARY KEY
);

CREATE TABLE analytics_write_offs (
    event_id    uuid PRIMARY KEY,
    product_id uuid        NOT NULL,
    reason     text        NOT NULL,
    quantity   bigint      NOT NULL CHECK (quantity > 0),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX analytics_write_offs_period_idx ON analytics_write_offs (occurred_at, product_id);

CREATE TABLE analytics_order_shortages (
    event_id    uuid        NOT NULL,
    product_id uuid        NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (event_id, product_id)
);
CREATE INDEX analytics_order_shortages_period_idx ON analytics_order_shortages (occurred_at, product_id);

CREATE TABLE analytics_consumption (
    event_id    uuid PRIMARY KEY,
    product_id uuid        NOT NULL,
    quantity   bigint      NOT NULL CHECK (quantity > 0),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX analytics_consumption_period_idx ON analytics_consumption (occurred_at, product_id);

-- Минимальное состояние склада и рецептур, нужное для восстановления интервалов стоп-листа.
CREATE TABLE analytics_batches (
    id         uuid PRIMARY KEY,
    product_id uuid        NOT NULL,
    quantity   bigint      NOT NULL CHECK (quantity >= 0),
    expires_at timestamptz NOT NULL
);
CREATE INDEX analytics_batches_product_idx ON analytics_batches (product_id, expires_at);

CREATE TABLE analytics_dishes (
    id uuid PRIMARY KEY
);

CREATE TABLE analytics_recipe_ingredients (
    dish_id    uuid   NOT NULL REFERENCES analytics_dishes(id) ON DELETE CASCADE,
    product_id uuid   NOT NULL,
    quantity   bigint NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (dish_id, product_id)
);
CREATE INDEX analytics_recipe_product_idx ON analytics_recipe_ingredients (product_id, dish_id);

CREATE TABLE analytics_stop_intervals (
    dish_id    uuid        NOT NULL,
    product_id uuid        NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at   timestamptz,
    PRIMARY KEY (dish_id, product_id, started_at),
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);
CREATE UNIQUE INDEX analytics_stop_intervals_open_idx
    ON analytics_stop_intervals (dish_id, product_id) WHERE ended_at IS NULL;
CREATE INDEX analytics_stop_intervals_period_idx
    ON analytics_stop_intervals (product_id, started_at, ended_at);

CREATE TABLE analytics_projection_clock (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    advanced_at timestamptz NOT NULL
);

-- +goose Down

DROP TABLE analytics_projection_clock;
DROP TABLE analytics_stop_intervals;
DROP TABLE analytics_recipe_ingredients;
DROP TABLE analytics_dishes;
DROP TABLE analytics_batches;
DROP TABLE analytics_consumption;
DROP TABLE analytics_order_shortages;
DROP TABLE analytics_write_offs;
DROP TABLE analytics_applied_events;
