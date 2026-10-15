-- +goose Up

-- Настройки генератора событий: единственная строка. Техническое состояние,
-- событий для него нет. Генератор читает настройки через API.
CREATE TABLE generator_settings (
    -- ограничение на значение не даёт создать вторую строку
    id                    boolean     PRIMARY KEY DEFAULT true CHECK (id),
    running               boolean     NOT NULL DEFAULT false,
    deliveries_per_minute integer     NOT NULL DEFAULT 6 CHECK (deliveries_per_minute BETWEEN 0 AND 600),
    orders_per_minute     integer     NOT NULL DEFAULT 30 CHECK (orders_per_minute BETWEEN 0 AND 600),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

INSERT INTO generator_settings DEFAULT VALUES;

-- +goose Down

DROP TABLE generator_settings;
