-- +goose Up

-- Поставщики компании и их ключи (ADR-014): техническая настройка доступа без событий, как подключение
-- Мигома. key_hash - SHA-256 ключа, NULL - ключ отозван. Поставщик не удаляется: его название
-- подставляется автором в историю поставок.
CREATE TABLE suppliers (
    id         uuid        PRIMARY KEY,
    company_id uuid        NOT NULL,
    name       text        NOT NULL,
    key_hash   bytea       UNIQUE,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- название уникально в компании без учёта регистра, как у точек
CREATE UNIQUE INDEX suppliers_company_name ON suppliers (company_id, lower(name));

CREATE TABLE supplier_points (
    supplier_id uuid NOT NULL REFERENCES suppliers (id),
    point_id    uuid NOT NULL,
    PRIMARY KEY (supplier_id, point_id)
);

-- +goose Down
DROP TABLE supplier_points;
DROP TABLE suppliers;
