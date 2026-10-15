-- +goose Up

-- Отмена заказа ищет его расход по партиям: события StockConsumed с этим order_id.
CREATE INDEX events_stock_consumed_order_idx ON events ((payload->>'order_id')) WHERE event_type = 'StockConsumed';
-- Расход для рекомендаций пропускает возвращённое при отмене: возврат ищется по тому же order_id.
CREATE INDEX events_stock_returned_order_idx ON events ((payload->>'order_id')) WHERE event_type = 'StockReturned';

-- +goose Down
DROP INDEX events_stock_returned_order_idx;
DROP INDEX events_stock_consumed_order_idx;
