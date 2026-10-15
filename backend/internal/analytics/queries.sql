-- name: WriteOffStatistics :many
SELECT product_id, reason,
       sum(quantity)::bigint AS quantity,
       count(*)::bigint AS operations
FROM analytics_write_offs
WHERE point_id = sqlc.arg(point_id) AND occurred_at >= sqlc.arg(from_time) AND occurred_at < sqlc.arg(to_time)
GROUP BY product_id, reason;

-- name: WriteOffDays :many
-- День - дата события в часовом поясе клиента; пояс проверяет сервис до запроса.
SELECT to_char(occurred_at AT TIME ZONE sqlc.arg(time_zone)::text, 'YYYY-MM-DD')::text AS day,
       product_id, reason,
       sum(quantity)::bigint AS quantity,
       count(*)::bigint AS operations
FROM analytics_write_offs
WHERE point_id = sqlc.arg(point_id) AND occurred_at >= sqlc.arg(from_time) AND occurred_at < sqlc.arg(to_time)
GROUP BY 1, product_id, reason
ORDER BY 1, product_id, reason;

-- name: ShortageStatistics :many
WITH products AS (
    SELECT os.product_id FROM analytics_order_shortages os
    WHERE os.point_id = sqlc.arg(point_id) AND os.occurred_at >= sqlc.arg(from_time) AND os.occurred_at < sqlc.arg(to_time)
    UNION
    SELECT si.product_id FROM analytics_stop_intervals si
    WHERE si.point_id = sqlc.arg(point_id) AND si.started_at < sqlc.arg(to_time)
      AND COALESCE(si.ended_at, sqlc.arg(to_time)) > sqlc.arg(from_time)
), rejected AS (
    SELECT os.product_id, count(*)::bigint AS rejected_orders
    FROM analytics_order_shortages os
    WHERE os.point_id = sqlc.arg(point_id) AND os.occurred_at >= sqlc.arg(from_time) AND os.occurred_at < sqlc.arg(to_time)
    GROUP BY os.product_id
), stopped AS (
    SELECT si.product_id,
           count(*) FILTER (
               WHERE si.started_at >= sqlc.arg(from_time) AND si.started_at < sqlc.arg(to_time)
           )::bigint AS stop_entries,
           -- незакрытый интервал длится до текущего момента, а не до конца периода: иначе период
           -- с концом в будущем засчитывал время, которого ещё не было
           COALESCE(sum(GREATEST(EXTRACT(EPOCH FROM (
               LEAST(COALESCE(si.ended_at, sqlc.arg(now_time)::timestamptz), sqlc.arg(to_time))
               - GREATEST(si.started_at, sqlc.arg(from_time))
           )), 0)), 0)::bigint AS stop_seconds
    FROM analytics_stop_intervals si
    WHERE si.point_id = sqlc.arg(point_id) AND si.started_at < sqlc.arg(to_time)
      AND COALESCE(si.ended_at, sqlc.arg(to_time)) > sqlc.arg(from_time)
    GROUP BY si.product_id
)
SELECT p.product_id,
       COALESCE(r.rejected_orders, 0)::bigint AS rejected_orders,
       COALESCE(s.stop_entries, 0)::bigint AS stop_entries,
       COALESCE(s.stop_seconds, 0)::bigint AS stop_seconds
FROM products p
LEFT JOIN rejected r USING (product_id)
LEFT JOIN stopped s USING (product_id);

-- name: ConsumptionSince :many
SELECT product_id, sum(quantity)::bigint AS quantity
FROM analytics_consumption
WHERE point_id = sqlc.arg(point_id) AND occurred_at >= sqlc.arg(from_time) AND occurred_at < sqlc.arg(to_time)
GROUP BY product_id;

-- name: FirstConsumptionSince :one
-- Самый ранний расход любого продукта в окне: от него считается, за какое время есть данные.
-- Расхода нет - возвращается конец окна.
SELECT COALESCE(min(occurred_at), sqlc.arg(to_time))::timestamptz AS first_at
FROM analytics_consumption
WHERE point_id = sqlc.arg(point_id) AND occurred_at >= sqlc.arg(from_time) AND occurred_at < sqlc.arg(to_time);
