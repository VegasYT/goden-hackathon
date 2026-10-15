-- +goose Up

-- Event Store: единственный источник истины (ADR-002).
CREATE TABLE events (
    position       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- по txid читатели отсекают события ещё не завершённых транзакций
    txid           xid8        NOT NULL DEFAULT pg_current_xact_id(),
    event_id       uuid        NOT NULL,
    stream_id      text        NOT NULL,
    stream_version bigint      NOT NULL CHECK (stream_version > 0),
    event_type     text        NOT NULL,
    schema_version integer     NOT NULL CHECK (schema_version > 0),
    payload        jsonb       NOT NULL,
    metadata       jsonb       NOT NULL DEFAULT '{}',
    correlation_id uuid,
    causation_id   uuid,
    occurred_at    timestamptz NOT NULL,
    CONSTRAINT events_event_id_key UNIQUE (event_id),
    CONSTRAINT events_stream_version_key UNIQUE (stream_id, stream_version)
);

CREATE INDEX events_txid_position_idx ON events (txid, position);

-- +goose StatementBegin
CREATE FUNCTION events_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'events are append-only: % is not allowed', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER events_no_update_delete
    BEFORE UPDATE OR DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION events_reject_change();

CREATE TRIGGER events_no_truncate
    BEFORE TRUNCATE ON events
    FOR EACH STATEMENT EXECUTE FUNCTION events_reject_change();

-- +goose Down

DROP TABLE events;
DROP FUNCTION events_reject_change();
