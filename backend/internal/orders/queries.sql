-- name: InsertOrder :exec
INSERT INTO orders_history (id, point_id, status, placed_at, state)
VALUES (sqlc.arg(id), sqlc.arg(point_id), sqlc.arg(status), sqlc.arg(placed_at), sqlc.arg(state));

-- name: OrderState :one
SELECT state FROM orders_history WHERE id = sqlc.arg(id);

-- name: UpdateOrder :exec
UPDATE orders_history SET status = sqlc.arg(status), state = sqlc.arg(state) WHERE id = sqlc.arg(id);

-- name: ListOrders :many
SELECT state
FROM orders_history
WHERE point_id = sqlc.arg(point_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
ORDER BY placed_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountOrders :one
SELECT count(*)
FROM orders_history
WHERE point_id = sqlc.arg(point_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status));
