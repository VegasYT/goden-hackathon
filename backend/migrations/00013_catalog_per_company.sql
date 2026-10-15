-- +goose Up

-- Каталог принадлежит компании (ADR-013). Строки, созданные до этого, получают нулевую
-- компанию и ни в одной компании не видны: в событиях, по которым они построены, компании нет.
ALTER TABLE catalog_products ADD COLUMN company_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE catalog_products ALTER COLUMN company_id DROP DEFAULT;
ALTER TABLE catalog_products DROP CONSTRAINT catalog_products_name_key;
-- название уникально в пределах компании
ALTER TABLE catalog_products ADD CONSTRAINT catalog_products_name_key UNIQUE (company_id, name_key);

ALTER TABLE catalog_dishes ADD COLUMN company_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE catalog_dishes ALTER COLUMN company_id DROP DEFAULT;
ALTER TABLE catalog_dishes DROP CONSTRAINT catalog_dishes_name_key;
ALTER TABLE catalog_dishes ADD CONSTRAINT catalog_dishes_name_key UNIQUE (company_id, name_key);

-- Блюда, выключенные на точке. Блюда без строки продаются на точке: новое блюдо включено везде.
CREATE TABLE catalog_disabled_dishes (
    point_id uuid NOT NULL,
    dish_id  uuid NOT NULL REFERENCES catalog_dishes (id),
    PRIMARY KEY (point_id, dish_id)
);

-- +goose Down

DROP TABLE catalog_disabled_dishes;
ALTER TABLE catalog_dishes DROP CONSTRAINT catalog_dishes_name_key;
ALTER TABLE catalog_dishes ADD CONSTRAINT catalog_dishes_name_key UNIQUE (name_key);
ALTER TABLE catalog_dishes DROP COLUMN company_id;
ALTER TABLE catalog_products DROP CONSTRAINT catalog_products_name_key;
ALTER TABLE catalog_products ADD CONSTRAINT catalog_products_name_key UNIQUE (name_key);
ALTER TABLE catalog_products DROP COLUMN company_id;
