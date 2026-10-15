-- +goose Up

-- Расход помнит заказ: отмена заказа удаляет его расход, отменённый заказ спросом не считается.
-- Прежние строки без заказа удалит перестроение проекции analytics (её версия выросла).
ALTER TABLE analytics_consumption ADD COLUMN order_id uuid;
CREATE INDEX analytics_consumption_order_idx ON analytics_consumption (order_id, product_id);

-- +goose Down
DROP INDEX analytics_consumption_order_idx;
ALTER TABLE analytics_consumption DROP COLUMN order_id;
