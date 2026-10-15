-- +goose Up

-- Ключ идемпотентности принадлежит автору: один и тот же ключ у разных авторов - разные команды.
-- Раньше ключ был общим на систему: запрос с ключом, занятым в другой компании, получал отказ,
-- а предсказуемый ключ можно было занять заранее.
ALTER TABLE command_results ADD COLUMN actor text NOT NULL DEFAULT '';

-- автор прежних команд берётся из их событий; у команды без событий он остаётся пустым,
-- и её повтор после обновления выполнится заново
UPDATE command_results r
SET actor = COALESCE((
    SELECT (e.metadata -> 'actor' ->> 'type') || ':' || (e.metadata -> 'actor' ->> 'id')
    FROM events e
    WHERE e.causation_id = r.command_id
    LIMIT 1), '');

ALTER TABLE command_results DROP CONSTRAINT command_results_pkey;
ALTER TABLE command_results ADD PRIMARY KEY (command_name, actor, idempotency_key);

-- +goose Down

DELETE FROM command_results a USING command_results b
WHERE a.command_name = b.command_name AND a.idempotency_key = b.idempotency_key
  AND (a.created_at, a.actor) > (b.created_at, b.actor);
ALTER TABLE command_results DROP CONSTRAINT command_results_pkey;
ALTER TABLE command_results ADD PRIMARY KEY (command_name, idempotency_key);
ALTER TABLE command_results DROP COLUMN actor;
