-- +goose Up

-- Остатки партий: write model из ADR-002. Обновляется в одной транзакции с событиями
-- складских потоков, по ней команды склада принимают решения. Источник истины - Event Store.
CREATE TABLE inventory_batches (
    id                uuid PRIMARY KEY,
    product_id        uuid        NOT NULL,
    delivery_id       uuid        NOT NULL,
    item_id           uuid        NOT NULL,
    received_quantity bigint      NOT NULL CHECK (received_quantity > 0),
    -- текущий остаток; ограничение - последняя линия защиты от ухода в минус
    quantity          bigint      NOT NULL CHECK (quantity >= 0),
    expires_at        timestamptz NOT NULL,
    received_at       timestamptz NOT NULL
);

-- порядок FEFO: сначала партии с ближайшим сроком годности
CREATE INDEX inventory_batches_fefo_idx ON inventory_batches (product_id, expires_at, received_at);

-- +goose Down

DROP TABLE inventory_batches;
