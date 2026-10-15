-- +goose Up

-- Когда по партии выдано предупреждение о приближении срока годности.
-- Предупреждение выдаётся один раз на партию (ADR-008).
ALTER TABLE inventory_batches ADD COLUMN expiry_warned_at timestamptz;

-- +goose Down

ALTER TABLE inventory_batches DROP COLUMN expiry_warned_at;
