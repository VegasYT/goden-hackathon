-- name: InsertSupplier :exec
INSERT INTO suppliers (id, company_id, name, key_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5);

-- name: UpdateSupplierName :execrows
UPDATE suppliers SET name = $3, updated_at = $4 WHERE id = $1 AND company_id = $2;

-- name: SetSupplierKey :execrows
UPDATE suppliers SET key_hash = sqlc.narg(key_hash), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND company_id = sqlc.arg(company_id);

-- name: GetSupplier :one
SELECT id, company_id, name, (key_hash IS NOT NULL)::boolean AS key_active, created_at
FROM suppliers WHERE id = $1 AND company_id = $2;

-- name: ListSuppliers :many
SELECT id, company_id, name, (key_hash IS NOT NULL)::boolean AS key_active, created_at
FROM suppliers WHERE company_id = $1 ORDER BY lower(name);

-- name: SupplierByKey :one
SELECT id, company_id, name FROM suppliers WHERE key_hash = $1;

-- name: SupplierNames :many
SELECT id, name FROM suppliers WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteSupplierPoints :exec
DELETE FROM supplier_points WHERE supplier_id = $1;

-- name: InsertSupplierPoints :exec
INSERT INTO supplier_points (supplier_id, point_id)
SELECT sqlc.arg(supplier_id), unnest(sqlc.arg(point_ids)::uuid[]);

-- name: ListSupplierPoints :many
SELECT supplier_id, point_id FROM supplier_points
WHERE supplier_id = ANY(sqlc.arg(supplier_ids)::uuid[]) ORDER BY supplier_id, point_id;

-- name: SupplierOnPoint :one
SELECT EXISTS (SELECT 1 FROM supplier_points WHERE supplier_id = $1 AND point_id = $2);

-- name: SupplierCompany :one
SELECT company_id FROM suppliers WHERE id = $1;
