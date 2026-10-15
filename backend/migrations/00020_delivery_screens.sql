-- +goose Up

-- Число позиций поставки в карантине без решения: по нему считается сводка. Столбец заполняет
-- проекция; её новая версия перестраивает таблицу, значение по умолчанию живёт до перестроения.
ALTER TABLE inventory_deliveries ADD COLUMN open_quarantine_items integer NOT NULL DEFAULT 0;

-- Логин пользователя из токена: в событиях автор записан логином, по нему в ответах
-- подставляется имя. Пустой, пока пользователь не запросил профиль после этой миграции.
ALTER TABLE organization_users ADD COLUMN login text NOT NULL DEFAULT '';
CREATE INDEX organization_users_login_idx ON organization_users (login) WHERE login <> '';

-- +goose Down

DROP INDEX organization_users_login_idx;
ALTER TABLE organization_users DROP COLUMN login;
ALTER TABLE inventory_deliveries DROP COLUMN open_quarantine_items;
