-- +goose Up

-- Проекция аналитики на каждое событие ищет партии, срок которых истёк с прошлого события.
CREATE INDEX analytics_batches_expiry_idx ON analytics_batches (expires_at) WHERE quantity > 0;

-- Диспетчер уведомлений удаляет старые завершённые записи.
CREATE INDEX notification_deliveries_finished_idx ON notification_deliveries (created_at)
    WHERE status IN ('sent', 'failed');

-- +goose Down

DROP INDEX notification_deliveries_finished_idx;
DROP INDEX analytics_batches_expiry_idx;
