-- name: SaveHotOffer :exec
INSERT INTO recommendations_hot_offers (point_id, dish_id, mode, comment, set_by_type, set_by_id, set_at, hidden_until)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (point_id, dish_id) DO UPDATE
SET mode = EXCLUDED.mode, comment = EXCLUDED.comment, set_by_type = EXCLUDED.set_by_type,
    set_by_id = EXCLUDED.set_by_id, set_at = EXCLUDED.set_at, hidden_until = EXCLUDED.hidden_until;

-- name: DeleteHotOffer :exec
DELETE FROM recommendations_hot_offers WHERE point_id = $1 AND dish_id = $2;

-- name: GetHotOffer :one
SELECT mode, comment, hidden_until FROM recommendations_hot_offers WHERE point_id = $1 AND dish_id = $2;

-- name: ListHotOffers :many
-- Действующие ручные настройки точки в порядке изменения: истёкшее скрытие уже не действует.
SELECT dish_id, mode, comment, set_by_type, set_by_id, set_at, hidden_until
FROM recommendations_hot_offers
WHERE point_id = sqlc.arg(point_id) AND (hidden_until IS NULL OR hidden_until > sqlc.arg(now)::timestamptz)
ORDER BY set_at, dish_id;
