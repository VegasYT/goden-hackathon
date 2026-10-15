-- name: ReleaseEmail :exec
UPDATE organization_users
SET email = '', email_key = ''
WHERE email_key = $1 AND id <> $2;

-- name: UpsertUser :exec
INSERT INTO organization_users (id, email, email_key, name, login)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET email = excluded.email, email_key = excluded.email_key, name = excluded.name, login = excluded.login;

-- name: InsertUserIfNew :execrows
INSERT INTO organization_users (id, email, email_key, name, login)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO NOTHING;

-- name: ListNamesByLogin :many
SELECT login, name
FROM organization_users
WHERE login <> '' AND login = ANY(sqlc.arg(logins)::text[]) AND name <> '';

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM organization_users WHERE id = $1);

-- name: GetUserByEmail :one
SELECT id, email, name
FROM organization_users
WHERE email_key = $1;

-- name: InsertCompany :exec
INSERT INTO organization_companies (id, name, owner_id)
VALUES ($1, $2, $3);

-- name: RenameCompany :exec
UPDATE organization_companies SET name = $2 WHERE id = $1;

-- name: SetCompanyOwner :exec
UPDATE organization_companies SET owner_id = $2 WHERE id = $1;

-- name: DisableCompany :exec
UPDATE organization_companies SET active = false WHERE id = $1;

-- name: GetCompany :one
SELECT id, name, owner_id, active
FROM organization_companies
WHERE id = $1;

-- name: ListCompaniesOfOwner :many
SELECT id, name, owner_id, active
FROM organization_companies
WHERE owner_id = $1
ORDER BY id;

-- name: InsertPoint :exec
INSERT INTO organization_points (id, company_id, name, name_key)
VALUES ($1, $2, $3, $4);

-- name: RenamePoint :exec
UPDATE organization_points SET name = $2, name_key = $3 WHERE id = $1;

-- name: DisablePoint :exec
UPDATE organization_points SET active = false WHERE id = $1;

-- name: SetPointTelegramChat :exec
UPDATE organization_points SET telegram_chat_id = sqlc.narg(chat_id) WHERE id = sqlc.arg(id);

-- name: GetPoint :one
SELECT id, company_id, name, active, telegram_chat_id
FROM organization_points
WHERE id = $1;

-- name: ListPointsOfCompany :many
SELECT id, company_id, name, active, telegram_chat_id
FROM organization_points
WHERE company_id = $1
ORDER BY id;

-- name: InsertAssignment :exec
INSERT INTO organization_assignments (point_id, user_id, role)
VALUES ($1, $2, $3);

-- name: UpdateAssignmentRole :exec
UPDATE organization_assignments SET role = $3 WHERE point_id = $1 AND user_id = $2;

-- name: DeleteAssignment :exec
DELETE FROM organization_assignments WHERE point_id = $1 AND user_id = $2;

-- name: GetAssignmentRole :one
SELECT role
FROM organization_assignments
WHERE point_id = $1 AND user_id = $2;

-- name: ListStaff :many
SELECT a.user_id, a.role, coalesce(u.email, '')::text AS email, coalesce(u.name, '')::text AS name
FROM organization_assignments a
LEFT JOIN organization_users u ON u.id = a.user_id
WHERE a.point_id = $1
ORDER BY name, a.user_id;

-- name: GetStaff :one
SELECT a.user_id, a.role, coalesce(u.email, '')::text AS email, coalesce(u.name, '')::text AS name
FROM organization_assignments a
LEFT JOIN organization_users u ON u.id = a.user_id
WHERE a.point_id = $1 AND a.user_id = $2;

-- name: ListOwnedPoints :many
SELECT p.id, p.company_id, p.name, p.active, c.name AS company_name
FROM organization_points p
JOIN organization_companies c ON c.id = p.company_id
WHERE c.owner_id = $1
ORDER BY p.id;

-- сотрудник видит только работающие точки работающих компаний
-- name: ListAssignedPoints :many
SELECT p.id, p.company_id, p.name, p.active, c.name AS company_name, a.role
FROM organization_assignments a
JOIN organization_points p ON p.id = a.point_id
JOIN organization_companies c ON c.id = p.company_id
WHERE a.user_id = $1 AND p.active AND c.active
ORDER BY p.id;

-- name: WorksInCompany :one
SELECT EXISTS (
    SELECT 1
    FROM organization_assignments a
    JOIN organization_points p ON p.id = a.point_id
    WHERE a.user_id = $1 AND p.company_id = $2 AND p.active
);
