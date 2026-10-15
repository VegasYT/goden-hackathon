-- +goose Up

-- Проекции для списков. Источник истины - события; таблицы пересоздаются движком проекций.

-- Поставки: состояние целиком лежит в state, отдельные столбцы нужны только фильтрам и сортировке.
CREATE TABLE inventory_deliveries (
    id              uuid PRIMARY KEY,
    number          text        NOT NULL,
    status          text        NOT NULL,
    delivered_at    timestamptz NOT NULL,
    -- есть позиция с карантинным количеством, по которому ещё нет решения
    open_quarantine boolean     NOT NULL,
    state           jsonb       NOT NULL
);

CREATE INDEX inventory_deliveries_status_idx ON inventory_deliveries (status, delivered_at DESC, id DESC);
CREATE INDEX inventory_deliveries_quarantine_idx ON inventory_deliveries (delivered_at DESC, id DESC) WHERE open_quarantine;

-- История заказов: принятые и отклонённые.
CREATE TABLE orders_history (
    id        uuid PRIMARY KEY,
    status    text        NOT NULL,
    placed_at timestamptz NOT NULL,
    state     jsonb       NOT NULL
);

CREATE INDEX orders_history_status_idx ON orders_history (status, placed_at DESC, id DESC);

-- +goose Down

DROP TABLE orders_history;
DROP TABLE inventory_deliveries;
