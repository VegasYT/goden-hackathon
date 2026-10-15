-- +goose Up

-- Конечное состояние недоставленного уведомления: Telegram отклонил его окончательно
-- (чат не найден, бот не состоит в чате) или оно устарело, пока отправка не удавалась.
-- Без него запись с неверным чатом точки повторялась бесконечно и занимала диспетчер.
ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_status_check;
ALTER TABLE notification_deliveries
    ADD CONSTRAINT notification_deliveries_status_check CHECK (status IN ('pending', 'processing', 'sent', 'failed'));

-- +goose Down

UPDATE notification_deliveries SET status = 'pending', next_attempt_at = created_at WHERE status = 'failed';
ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_status_check;
ALTER TABLE notification_deliveries
    ADD CONSTRAINT notification_deliveries_status_check CHECK (status IN ('pending', 'processing', 'sent'));
