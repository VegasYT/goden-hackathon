-- +goose Up

-- Результаты команд с ключом идемпотентности. Техническое состояние стороны записи:
-- из событий не восстанавливается и бизнес-данных не содержит.
CREATE TABLE command_results (
    command_name    text        NOT NULL,
    idempotency_key text        NOT NULL,
    -- по хешу запроса отличаем повтор от чужого запроса с тем же ключом
    request_hash    text        NOT NULL,
    command_id      uuid        NOT NULL,
    -- заполняется в той же транзакции, что и события команды
    result          jsonb,
    created_at      timestamptz NOT NULL,
    PRIMARY KEY (command_name, idempotency_key)
);

-- +goose Down

DROP TABLE command_results;
