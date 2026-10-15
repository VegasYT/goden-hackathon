-- +goose Up

-- Отключённая точка не продаёт: её блюда не стоят в стоп-листе, время дефицита не растёт.
ALTER TABLE analytics_points ADD COLUMN active boolean NOT NULL DEFAULT true;

-- +goose Down

ALTER TABLE analytics_points DROP COLUMN active;
