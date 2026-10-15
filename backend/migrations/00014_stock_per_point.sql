-- +goose Up

-- Склад принадлежит точке (ADR-013). Строки, созданные до этого, получают нулевую точку
-- и ни на одной точке не видны: в событиях, по которым они построены, точки нет.
ALTER TABLE inventory_batches ADD COLUMN point_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE inventory_batches ALTER COLUMN point_id DROP DEFAULT;
DROP INDEX inventory_batches_fefo_idx;
-- порядок FEFO в границах точки
CREATE INDEX inventory_batches_fefo_idx ON inventory_batches (point_id, product_id, expires_at, received_at);

-- Проекции списков: движок перестроит их сам по новой версии проекции.
ALTER TABLE inventory_deliveries ADD COLUMN point_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE inventory_deliveries ALTER COLUMN point_id DROP DEFAULT;
DROP INDEX inventory_deliveries_status_idx;
DROP INDEX inventory_deliveries_quarantine_idx;
CREATE INDEX inventory_deliveries_status_idx ON inventory_deliveries (point_id, status, delivered_at DESC, id DESC);
CREATE INDEX inventory_deliveries_quarantine_idx ON inventory_deliveries (point_id, delivered_at DESC, id DESC) WHERE open_quarantine;

ALTER TABLE orders_history ADD COLUMN point_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE orders_history ALTER COLUMN point_id DROP DEFAULT;
DROP INDEX orders_history_status_idx;
CREATE INDEX orders_history_status_idx ON orders_history (point_id, status, placed_at DESC, id DESC);

-- Аналитика по точкам. Таблицы - проекция: очищаются здесь и перестраиваются движком
-- по новой версии проекции, поэтому столбцы добавляются без значений по умолчанию.
TRUNCATE analytics_applied_events, analytics_write_offs, analytics_order_shortages, analytics_consumption,
    analytics_batches, analytics_dishes, analytics_recipe_ingredients, analytics_stop_intervals, analytics_projection_clock;

ALTER TABLE analytics_write_offs ADD COLUMN point_id uuid NOT NULL;
ALTER TABLE analytics_order_shortages ADD COLUMN point_id uuid NOT NULL;
ALTER TABLE analytics_consumption ADD COLUMN point_id uuid NOT NULL;
ALTER TABLE analytics_batches ADD COLUMN point_id uuid NOT NULL;
ALTER TABLE analytics_dishes ADD COLUMN company_id uuid NOT NULL;
DROP INDEX analytics_write_offs_period_idx;
DROP INDEX analytics_order_shortages_period_idx;
DROP INDEX analytics_consumption_period_idx;
DROP INDEX analytics_batches_product_idx;
CREATE INDEX analytics_write_offs_period_idx ON analytics_write_offs (point_id, occurred_at, product_id);
CREATE INDEX analytics_order_shortages_period_idx ON analytics_order_shortages (point_id, occurred_at, product_id);
CREATE INDEX analytics_consumption_period_idx ON analytics_consumption (point_id, occurred_at, product_id);
CREATE INDEX analytics_batches_product_idx ON analytics_batches (point_id, product_id, expires_at);

-- интервалы стоп-листа ведутся отдельно по каждой точке: остатки у точек свои
ALTER TABLE analytics_stop_intervals ADD COLUMN point_id uuid NOT NULL;
ALTER TABLE analytics_stop_intervals DROP CONSTRAINT analytics_stop_intervals_pkey;
ALTER TABLE analytics_stop_intervals ADD PRIMARY KEY (point_id, dish_id, product_id, started_at);
DROP INDEX analytics_stop_intervals_open_idx;
DROP INDEX analytics_stop_intervals_period_idx;
CREATE UNIQUE INDEX analytics_stop_intervals_open_idx
    ON analytics_stop_intervals (point_id, dish_id, product_id) WHERE ended_at IS NULL;
CREATE INDEX analytics_stop_intervals_period_idx
    ON analytics_stop_intervals (point_id, product_id, started_at, ended_at);

-- точки компаний и блюда, выключенные на точке: по ним блюдо относится к точкам
CREATE TABLE analytics_points (
    id         uuid PRIMARY KEY,
    company_id uuid NOT NULL
);

CREATE TABLE analytics_disabled_dishes (
    point_id uuid NOT NULL,
    dish_id  uuid NOT NULL,
    PRIMARY KEY (point_id, dish_id)
);

-- +goose Down

DROP TABLE analytics_disabled_dishes;
DROP TABLE analytics_points;
TRUNCATE analytics_applied_events, analytics_write_offs, analytics_order_shortages, analytics_consumption,
    analytics_batches, analytics_dishes, analytics_recipe_ingredients, analytics_stop_intervals, analytics_projection_clock;
DROP INDEX analytics_stop_intervals_period_idx;
DROP INDEX analytics_stop_intervals_open_idx;
ALTER TABLE analytics_stop_intervals DROP CONSTRAINT analytics_stop_intervals_pkey;
ALTER TABLE analytics_stop_intervals DROP COLUMN point_id;
ALTER TABLE analytics_stop_intervals ADD PRIMARY KEY (dish_id, product_id, started_at);
CREATE UNIQUE INDEX analytics_stop_intervals_open_idx ON analytics_stop_intervals (dish_id, product_id) WHERE ended_at IS NULL;
CREATE INDEX analytics_stop_intervals_period_idx ON analytics_stop_intervals (product_id, started_at, ended_at);
ALTER TABLE analytics_dishes DROP COLUMN company_id;
ALTER TABLE analytics_batches DROP COLUMN point_id;
ALTER TABLE analytics_consumption DROP COLUMN point_id;
ALTER TABLE analytics_order_shortages DROP COLUMN point_id;
ALTER TABLE analytics_write_offs DROP COLUMN point_id;
CREATE INDEX analytics_write_offs_period_idx ON analytics_write_offs (occurred_at, product_id);
CREATE INDEX analytics_order_shortages_period_idx ON analytics_order_shortages (occurred_at, product_id);
CREATE INDEX analytics_consumption_period_idx ON analytics_consumption (occurred_at, product_id);
CREATE INDEX analytics_batches_product_idx ON analytics_batches (product_id, expires_at);

DROP INDEX orders_history_status_idx;
CREATE INDEX orders_history_status_idx ON orders_history (status, placed_at DESC, id DESC);
ALTER TABLE orders_history DROP COLUMN point_id;
DROP INDEX inventory_deliveries_quarantine_idx;
DROP INDEX inventory_deliveries_status_idx;
CREATE INDEX inventory_deliveries_status_idx ON inventory_deliveries (status, delivered_at DESC, id DESC);
CREATE INDEX inventory_deliveries_quarantine_idx ON inventory_deliveries (delivered_at DESC, id DESC) WHERE open_quarantine;
ALTER TABLE inventory_deliveries DROP COLUMN point_id;
DROP INDEX inventory_batches_fefo_idx;
CREATE INDEX inventory_batches_fefo_idx ON inventory_batches (product_id, expires_at, received_at);
ALTER TABLE inventory_batches DROP COLUMN point_id;
