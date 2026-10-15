-- +goose Up

-- Позиции асинхронных проекций в общем порядке чтения событий (txid, position).
-- Строка меняется в одной транзакции с данными проекции, поэтому событие применяется
-- ровно один раз; блокировка строки на время порции не даёт двум экземплярам процесса
-- применить одно событие дважды.
CREATE TABLE projection_checkpoints (
    name       text PRIMARY KEY,
    -- версия логики проекции: при несовпадении с кодом проекция перестраивается с нуля
    version    integer     NOT NULL,
    txid       xid8        NOT NULL DEFAULT '0',
    position   bigint      NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL
);

-- +goose Down

DROP TABLE projection_checkpoints;
