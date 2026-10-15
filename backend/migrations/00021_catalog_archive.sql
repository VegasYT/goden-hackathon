-- +goose Up

-- Архив вместо удаления: архивный продукт и блюдо остаются в каталоге, их название занято.
ALTER TABLE catalog_products ADD COLUMN archived boolean NOT NULL DEFAULT false;
ALTER TABLE catalog_dishes ADD COLUMN archived boolean NOT NULL DEFAULT false;
-- архивное блюдо не продаётся ни на одной точке и дефицита не создаёт; проекцию заполняет worker
ALTER TABLE analytics_dishes ADD COLUMN archived boolean NOT NULL DEFAULT false;

-- Продукты, которые хоть раз были в поставке любого статуса: у них уже нельзя менять единицу.
-- Таблица записи склада, её заполняет регистрация поставки; уже записанные поставки
-- переносятся из событий.
CREATE TABLE inventory_delivered_products (
    product_id uuid PRIMARY KEY
);
INSERT INTO inventory_delivered_products (product_id)
SELECT DISTINCT (item ->> 'product_id')::uuid
FROM events, jsonb_array_elements(payload -> 'items') AS item
WHERE event_type = 'DeliveryRegistered'
ON CONFLICT DO NOTHING;

-- +goose Down

DROP TABLE inventory_delivered_products;
ALTER TABLE analytics_dishes DROP COLUMN archived;
ALTER TABLE catalog_dishes DROP COLUMN archived;
ALTER TABLE catalog_products DROP COLUMN archived;
