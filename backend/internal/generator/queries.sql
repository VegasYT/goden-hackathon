-- name: GetPointSettings :one
SELECT running, deliveries_per_minute, orders_per_minute, updated_at
FROM generator_point_settings
WHERE point_id = $1;

-- name: UpsertPointSettings :one
INSERT INTO generator_point_settings (point_id, company_id, running, deliveries_per_minute, orders_per_minute, updated_at)
VALUES (sqlc.arg(point_id), sqlc.arg(company_id), sqlc.arg(running), sqlc.arg(deliveries_per_minute),
        sqlc.arg(orders_per_minute), sqlc.arg(updated_at))
ON CONFLICT (point_id) DO UPDATE SET
    running = EXCLUDED.running, deliveries_per_minute = EXCLUDED.deliveries_per_minute,
    orders_per_minute = EXCLUDED.orders_per_minute, updated_at = EXCLUDED.updated_at
RETURNING running, deliveries_per_minute, orders_per_minute, updated_at;

-- Точки, на которых генератор включён: по ним он создаёт поставки и заказы.
-- name: ListRunningPoints :many
SELECT point_id, company_id, deliveries_per_minute, orders_per_minute
FROM generator_point_settings
WHERE running
ORDER BY point_id;

-- name: RunsOnPoint :one
SELECT EXISTS (SELECT 1 FROM generator_point_settings WHERE point_id = $1 AND running);

-- name: RunsInCompany :one
SELECT EXISTS (SELECT 1 FROM generator_point_settings WHERE company_id = $1 AND running);
