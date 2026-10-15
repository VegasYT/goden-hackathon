-- name: InsertBatch :exec
INSERT INTO inventory_batches (id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, point_id)
VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8);

-- name: ListBatches :many
SELECT id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, expiry_warned_at, point_id, expired_noticed_at
FROM inventory_batches
WHERE point_id = sqlc.arg(point_id) AND product_id = sqlc.arg(product_id)
ORDER BY expires_at, received_at, id;

-- name: ListStockedBatches :many
-- Партии набора продуктов на точке с остатком, в порядке FEFO внутри каждого продукта.
-- Просроченные не отфильтрованы: их отсекает planConsumption.
SELECT id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, expiry_warned_at, point_id, expired_noticed_at
FROM inventory_batches
WHERE point_id = sqlc.arg(point_id)
  AND product_id = ANY (sqlc.arg(product_ids)::uuid[])
  AND quantity > 0
ORDER BY product_id, expires_at, received_at, id;

-- name: GetBatch :one
SELECT id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, expiry_warned_at, point_id, expired_noticed_at
FROM inventory_batches
WHERE id = $1;

-- name: ChangeBatchQuantity :exec
-- Расход, списание и корректировка меняют остаток этим запросом: delta со знаком.
UPDATE inventory_batches
SET quantity = quantity + sqlc.arg(delta)
WHERE id = sqlc.arg(id);

-- name: MarkBatchExpiredNoticed :exec
UPDATE inventory_batches
SET expired_noticed_at = sqlc.arg(noticed_at)
WHERE id = sqlc.arg(id);

-- name: MarkBatchWarned :exec
UPDATE inventory_batches
SET expiry_warned_at = sqlc.arg(warned_at)
WHERE id = sqlc.arg(id);

-- name: ListItemBatches :many
-- Партии позиции поставки от последней созданной к первой: в этом порядке
-- корректировка приёмки снимает с них количество.
SELECT id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, expiry_warned_at, point_id, expired_noticed_at
FROM inventory_batches
WHERE item_id = $1
ORDER BY received_at DESC, id DESC;

-- name: UpsertDelivery :exec
INSERT INTO inventory_deliveries (id, point_id, number, status, delivered_at, open_quarantine, open_quarantine_items, state)
VALUES (sqlc.arg(id), sqlc.arg(point_id), sqlc.arg(number), sqlc.arg(status), sqlc.arg(delivered_at), sqlc.arg(open_quarantine),
        sqlc.arg(open_quarantine_items), sqlc.arg(state))
ON CONFLICT (id) DO UPDATE SET
    number = EXCLUDED.number, status = EXCLUDED.status, delivered_at = EXCLUDED.delivered_at,
    open_quarantine = EXCLUDED.open_quarantine, open_quarantine_items = EXCLUDED.open_quarantine_items, state = EXCLUDED.state;

-- name: ListDeliveries :many
SELECT state
FROM inventory_deliveries
WHERE point_id = sqlc.arg(point_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (NOT sqlc.arg(open_quarantine_only)::boolean OR open_quarantine)
  -- упрощение: поиск по началу номера просматривает поставки точки; индекс по lower(number), если их станет много
  AND starts_with(lower(number), sqlc.arg(number_prefix)::text)
ORDER BY CASE WHEN sqlc.arg(oldest_first)::boolean THEN delivered_at END,
         CASE WHEN sqlc.arg(oldest_first)::boolean THEN id END,
         delivered_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountDeliveries :one
SELECT count(*)
FROM inventory_deliveries
WHERE point_id = sqlc.arg(point_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (NOT sqlc.arg(open_quarantine_only)::boolean OR open_quarantine)
  AND starts_with(lower(number), sqlc.arg(number_prefix)::text);

-- name: SummarizeDeliveries :one
SELECT count(*) AS total,
       count(*) FILTER (WHERE status = 'registered') AS registered,
       count(*) FILTER (WHERE status = 'approved') AS approved,
       COALESCE(sum(open_quarantine_items), 0)::bigint AS open_quarantine_items
FROM inventory_deliveries
WHERE point_id = sqlc.arg(point_id);

-- name: StockOfItems :many
-- Текущий остаток партий по позициям поставок.
SELECT item_id, sum(quantity)::bigint AS quantity
FROM inventory_batches
WHERE item_id = ANY(sqlc.arg(item_ids)::uuid[])
GROUP BY item_id;

-- name: DeliveriesOfBatches :many
-- Поставки партий по идентификаторам партий; пустые партии тоже есть в таблице.
SELECT id, delivery_id
FROM inventory_batches
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: StockLevels :many
-- Остатки продуктов на точке на момент now. Партии с нулевым остатком не учитываются.
SELECT product_id,
       COALESCE(sum(quantity) FILTER (WHERE expires_at > sqlc.arg(now)), 0)::bigint AS available_quantity,
       COALESCE(sum(quantity) FILTER (WHERE expires_at <= sqlc.arg(now)), 0)::bigint AS expired_quantity,
       count(*) AS batches,
       min(expires_at) FILTER (WHERE expires_at > sqlc.arg(now)) AS nearest_expires_at
FROM inventory_batches
WHERE point_id = sqlc.arg(point_id) AND product_id = ANY(sqlc.arg(product_ids)::uuid[]) AND quantity > 0
GROUP BY product_id;

-- name: ListExpiredBatchIDs :many
-- Кандидаты на отметку о просрочке: ещё не отмечены, иначе накопившаяся просрочка занимала бы
-- весь лимит прохода. Решение принимает команда под блокировкой.
SELECT id FROM inventory_batches
WHERE quantity > 0 AND expires_at <= sqlc.arg(now) AND expired_noticed_at IS NULL
ORDER BY expires_at, id
LIMIT sqlc.arg(max_rows);

-- name: ListBatchIDsToWarn :many
-- Кандидаты на предупреждение о сроке: в периоде предупреждения и ещё без предупреждения.
SELECT id FROM inventory_batches
WHERE quantity > 0 AND expires_at > sqlc.arg(now) AND expires_at <= sqlc.arg(until) AND expiry_warned_at IS NULL
ORDER BY expires_at, id
LIMIT sqlc.arg(max_rows);

-- name: CountExpiryState :one
SELECT count(*) FILTER (WHERE expires_at > sqlc.arg(now) AND expires_at <= sqlc.arg(until)) AS expiring,
       count(*) FILTER (WHERE expires_at <= sqlc.arg(now)) AS expired
FROM inventory_batches
WHERE quantity > 0;

-- name: ListExpiringBatches :many
-- Партии точки с остатком, срок которых истёк или истекает не позже until, от ближайшего срока.
SELECT id, product_id, delivery_id, item_id, received_quantity, quantity, expires_at, received_at, expiry_warned_at, point_id, expired_noticed_at
FROM inventory_batches
WHERE point_id = sqlc.arg(point_id) AND quantity > 0 AND expires_at <= sqlc.arg(until)
ORDER BY expires_at, received_at, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountExpiringBatches :one
SELECT count(*) FROM inventory_batches
WHERE point_id = sqlc.arg(point_id) AND quantity > 0 AND expires_at <= sqlc.arg(until);

-- name: MarkProductsDelivered :exec
-- Продукты поставки: после первой поставки у продукта нельзя менять единицу.
INSERT INTO inventory_delivered_products (product_id)
SELECT unnest(sqlc.arg(product_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: DeliveredProducts :many
SELECT product_id FROM inventory_delivered_products WHERE product_id = ANY (sqlc.arg(product_ids)::uuid[]);
