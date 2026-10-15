-- +goose Up

-- Пользователи, которые хотя бы раз обращались к системе: отражение данных Keycloak,
-- обновляется при обращении. Не бизнес-состояние, из событий не перестраивается.
CREATE TABLE organization_users (
    -- sub токена: не меняется при смене почты и имени
    id        text PRIMARY KEY,
    email     text NOT NULL,
    -- почта в нижнем регистре: по ней сотрудника находят при назначении
    email_key text NOT NULL,
    name      text NOT NULL
);

-- у сервисной учётной записи почты нет
CREATE UNIQUE INDEX organization_users_email_key ON organization_users (email_key) WHERE email_key <> '';

-- Синхронные таблицы организации: обновляются в одной транзакции с событиями.
-- Источник истины - Event Store, таблицы принадлежат модулю organization.
CREATE TABLE organization_companies (
    id       uuid PRIMARY KEY,
    name     text NOT NULL,
    owner_id text NOT NULL,
    active   boolean NOT NULL DEFAULT true
);

CREATE INDEX organization_companies_owner ON organization_companies (owner_id);

CREATE TABLE organization_points (
    id         uuid PRIMARY KEY,
    company_id uuid NOT NULL REFERENCES organization_companies (id),
    name       text NOT NULL,
    -- название в нижнем регистре: уникально в пределах компании
    name_key   text NOT NULL,
    active     boolean NOT NULL DEFAULT true,
    CONSTRAINT organization_points_name_key UNIQUE (company_id, name_key)
);

-- Назначение сотрудника на точку. Снятое назначение удаляется: история остаётся в событиях.
CREATE TABLE organization_assignments (
    point_id uuid NOT NULL REFERENCES organization_points (id),
    user_id  text NOT NULL,
    role     text NOT NULL CHECK (role IN ('manager', 'approver', 'cook', 'waiter')),
    PRIMARY KEY (point_id, user_id)
);

CREATE INDEX organization_assignments_user ON organization_assignments (user_id);

-- +goose Down

DROP TABLE organization_assignments;
DROP TABLE organization_points;
DROP TABLE organization_companies;
DROP TABLE organization_users;
