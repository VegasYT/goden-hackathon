-- +goose Up

-- Publisher подтверждает позицию только после ack Kafka. Повтор между ack и этой записью
-- допустим: consumers устраняют его по event_id.
CREATE TABLE kafka_publisher_checkpoints (
    name       text PRIMARY KEY,
    txid       xid8        NOT NULL DEFAULT '0',
    position   bigint      NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL
);

-- Inbox consumer хранится в PostgreSQL, а не в Kafka/Redis: после рестарта уже
-- обработанный event_id по-прежнему не вызывает handler второй раз.
CREATE TABLE kafka_consumed_events (
    consumer_name text        NOT NULL,
    event_id      uuid        NOT NULL,
    outcome       text        NOT NULL CHECK (outcome IN ('handled', 'error')),
    processed_at  timestamptz NOT NULL,
    PRIMARY KEY (consumer_name, event_id)
);

-- +goose Down

DROP TABLE kafka_consumed_events;
DROP TABLE kafka_publisher_checkpoints;
