-- name: OutletOnPoint :one
SELECT outlet_id FROM migom_points WHERE point_id = $1;

-- name: SetOutletOnPoint :exec
INSERT INTO migom_points (point_id, outlet_id, updated_at)
VALUES (sqlc.arg(point_id), sqlc.narg(outlet_id), sqlc.arg(updated_at))
ON CONFLICT (point_id) DO UPDATE SET outlet_id = EXCLUDED.outlet_id, updated_at = EXCLUDED.updated_at;

-- name: ListConnectedPoints :many
SELECT point_id, outlet_id::bigint AS outlet_id FROM migom_points WHERE outlet_id IS NOT NULL ORDER BY point_id;
