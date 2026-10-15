-- +goose Up

-- Чат Telegram точки: в него уходят уведомления о её сроках и автосписаниях (ADR-013).
ALTER TABLE organization_points ADD COLUMN telegram_chat_id bigint;

-- Адресат уведомления определяется при постановке в очередь. Ноль - общий чат окружения:
-- точка без своего чата и ошибки обработчиков.
ALTER TABLE notification_deliveries ADD COLUMN chat_id bigint NOT NULL DEFAULT 0;

-- Настройки генератора у точки заменяют единственную общую строку. Техническое состояние,
-- событий для него нет. Точка без строки: генератор остановлен, интенсивность по умолчанию.
DROP TABLE generator_settings;

CREATE TABLE generator_point_settings (
    point_id              uuid        PRIMARY KEY,
    -- компания точки: по ней генератор получает каталог и права на его наполнение
    company_id            uuid        NOT NULL,
    running               boolean     NOT NULL,
    deliveries_per_minute integer     NOT NULL CHECK (deliveries_per_minute BETWEEN 0 AND 600),
    orders_per_minute     integer     NOT NULL CHECK (orders_per_minute BETWEEN 0 AND 600),
    updated_at            timestamptz NOT NULL
);

CREATE INDEX generator_point_settings_running ON generator_point_settings (company_id) WHERE running;

-- +goose Down

DROP TABLE generator_point_settings;

CREATE TABLE generator_settings (
    id                    boolean     PRIMARY KEY DEFAULT true CHECK (id),
    running               boolean     NOT NULL DEFAULT false,
    deliveries_per_minute integer     NOT NULL DEFAULT 6 CHECK (deliveries_per_minute BETWEEN 0 AND 600),
    orders_per_minute     integer     NOT NULL DEFAULT 30 CHECK (orders_per_minute BETWEEN 0 AND 600),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

INSERT INTO generator_settings DEFAULT VALUES;

ALTER TABLE notification_deliveries DROP COLUMN chat_id;
ALTER TABLE organization_points DROP COLUMN telegram_chat_id;
