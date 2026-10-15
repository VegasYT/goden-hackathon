-- +goose Up

-- Когда партия отмечена просроченной (событие BatchExpired). Отметка выдаётся один раз на партию;
-- остаток просроченной партии не меняется, пока её не спишут вручную.
ALTER TABLE inventory_batches ADD COLUMN expired_noticed_at timestamptz;

-- Уведомление о просроченной партии вместо уведомления об автосписании; старые записи остаются.
ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_kind_check;
ALTER TABLE notification_deliveries ADD CONSTRAINT notification_deliveries_kind_check
    CHECK (kind IN ('expiry_warning', 'expired_write_off', 'batch_expired', 'consumer_error'));

-- +goose Down

DELETE FROM notification_deliveries WHERE kind = 'batch_expired';
ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_kind_check;
ALTER TABLE notification_deliveries ADD CONSTRAINT notification_deliveries_kind_check
    CHECK (kind IN ('expiry_warning', 'expired_write_off', 'consumer_error'));

ALTER TABLE inventory_batches DROP COLUMN expired_noticed_at;
