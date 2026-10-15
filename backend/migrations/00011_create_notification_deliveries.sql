-- +goose Up

CREATE TABLE notification_deliveries (
    source_key      text        PRIMARY KEY,
    kind            text        NOT NULL CHECK (kind IN ('expiry_warning', 'expired_write_off', 'consumer_error')),
    body            text        NOT NULL CHECK (body <> ''),
    status          text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent')),
    attempts        integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL,
    lease_until     timestamptz,
    last_error      text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL,
    sent_at         timestamptz,
    CHECK ((status = 'processing') = (lease_until IS NOT NULL)),
    CHECK ((status = 'sent') = (sent_at IS NOT NULL))
);

CREATE INDEX notification_deliveries_ready_idx
    ON notification_deliveries (next_attempt_at, created_at)
    WHERE status IN ('pending', 'processing');

-- +goose Down

DROP TABLE notification_deliveries;
