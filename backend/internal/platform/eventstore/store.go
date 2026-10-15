// Package eventstore хранит доменные события в PostgreSQL (ADR-002).
//
// Пакет работает с содержимым события как с готовым JSON и ничего не знает
// о доменных типах: их сериализует реестр из registry.go.
package eventstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrVersionConflict - поток изменился с момента, когда вызывающая сторона прочитала его версию.
	// Транзакция после этой ошибки непригодна: команда повторяется целиком в новой транзакции.
	ErrVersionConflict = errors.New("eventstore: stream version conflict")
	// ErrDuplicateEvent - событие с таким идентификатором уже записано.
	ErrDuplicateEvent = errors.New("eventstore: duplicate event id")
	// ErrInvalidInput - вызов с некорректными аргументами, до базы он не доходит.
	ErrInvalidInput = errors.New("eventstore: invalid input")
)

const (
	streamVersionConstraint = "events_stream_version_key"
	eventIDConstraint       = "events_event_id_key"
	uniqueViolation         = "23505"
)

// NewEvent - событие, которое вызывающая сторона добавляет в поток.
type NewEvent struct {
	ID            uuid.UUID
	Type          string
	SchemaVersion int
	Payload       json.RawMessage
	Metadata      json.RawMessage // необязательные
	CorrelationID uuid.UUID       // uuid.Nil, если нет
	CausationID   uuid.UUID       // uuid.Nil, если нет
	OccurredAt    time.Time       // из clock.Clock, не из базы
}

// Event - записанное событие.
type Event struct {
	NewEvent
	StreamID string
	Version  int64  // версия в потоке, с 1
	Position int64  // глобальный номер
	TxID     uint64 // транзакция, записавшая событие
}

// Checkpoint - позиция читателя в общем порядке событий (txid, position).
type Checkpoint struct {
	TxID     uint64
	Position int64
}

// Checkpoint возвращает позицию события: её читатель передаёт в ReadAll, чтобы продолжить после него.
func (e Event) Checkpoint() Checkpoint { return Checkpoint{TxID: e.TxID, Position: e.Position} }

// Querier удовлетворяют и pgx.Tx, и *pgxpool.Pool: читать можно внутри транзакции команды и снаружи.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Append добавляет события в поток, если его текущая версия равна expectedVersion
// (0 для нового потока), и возвращает новую версию потока.
//
// Запись идёт в транзакции вызывающей стороны: события нескольких потоков
// и связанные с ними таблицы фиксируются вместе или не фиксируются вовсе.
func Append(ctx context.Context, tx pgx.Tx, streamID string, expectedVersion int64, events ...NewEvent) (int64, error) {
	if err := validate(streamID, expectedVersion, events); err != nil {
		return 0, err
	}

	current, err := StreamVersion(ctx, tx, streamID)
	if err != nil {
		return 0, err
	}
	// ловит и отставание, и опережение: вставка с опережающей версией оставила бы пропуск
	if current != expectedVersion {
		return 0, fmt.Errorf("stream %q: expected version %d, actual %d: %w",
			streamID, expectedVersion, current, ErrVersionConflict)
	}

	batch := &pgx.Batch{}
	for i, e := range events {
		batch.Queue(`
			INSERT INTO events (event_id, stream_id, stream_version, event_type, schema_version,
			                    payload, metadata, correlation_id, causation_id, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			e.ID, streamID, expectedVersion+int64(i)+1, e.Type, e.SchemaVersion,
			[]byte(e.Payload), metadataOrEmpty(e.Metadata), nullableUUID(e.CorrelationID), nullableUUID(e.CausationID),
			e.OccurredAt.UTC(),
		)
	}
	// гонку между проверкой версии и вставкой закрывает ограничение уникальности
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return 0, mapInsertError(streamID, err)
	}
	return expectedVersion + int64(len(events)), nil
}

// StreamVersion возвращает текущую версию потока, 0 для несуществующего.
func StreamVersion(ctx context.Context, q Querier, streamID string) (int64, error) {
	rows, err := q.Query(ctx, `SELECT COALESCE(max(stream_version), 0) FROM events WHERE stream_id = $1`, streamID)
	if err != nil {
		return 0, fmt.Errorf("eventstore: read stream version: %w", err)
	}
	version, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("eventstore: read stream version: %w", err)
	}
	return version, nil
}

// ReadStream возвращает события потока по возрастанию версии, начиная с fromVersion.
func ReadStream(ctx context.Context, q Querier, streamID string, fromVersion int64) ([]Event, error) {
	rows, err := q.Query(ctx, selectEvents+`
		WHERE stream_id = $1 AND stream_version >= $2
		ORDER BY stream_version`, streamID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("eventstore: read stream: %w", err)
	}
	return collect(rows)
}

// ReadFirst возвращает первые события потоков streamIDs одним запросом, без порядка.
// Потока без событий в ответе нет.
func ReadFirst(ctx context.Context, q Querier, streamIDs []string) ([]Event, error) {
	if len(streamIDs) == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx, selectEvents+`
		WHERE stream_id = ANY($1) AND stream_version = 1`, streamIDs)
	if err != nil {
		return nil, fmt.Errorf("eventstore: read first events: %w", err)
	}
	return collect(rows)
}

// ReadStreamBackward возвращает до limit событий потока с версией меньше beforeVersion,
// от последних к ранним. Так длинный поток читается постранично с конца.
func ReadStreamBackward(ctx context.Context, q Querier, streamID string, beforeVersion int64, limit int) ([]Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive: %w", ErrInvalidInput)
	}
	rows, err := q.Query(ctx, selectEvents+`
		WHERE stream_id = $1 AND stream_version < $2
		ORDER BY stream_version DESC
		LIMIT $3`, streamID, beforeVersion, limit)
	if err != nil {
		return nil, fmt.Errorf("eventstore: read stream backward: %w", err)
	}
	return collect(rows)
}

// ReadAll возвращает до limit событий всех потоков, идущих после позиции after,
// в порядке (txid, position). Нулевой Checkpoint читает с начала.
//
// Возвращаются только события транзакций старше самой старой незавершённой:
// ниже этой границы новые события появиться не могут, поэтому читатель,
// который продолжает с позиции последнего полученного события, ничего не пропускает.
func ReadAll(ctx context.Context, q Querier, after Checkpoint, limit int) ([]Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive: %w", ErrInvalidInput)
	}
	// xid8 передаётся текстом, чтобы не зависеть от кодека драйвера
	rows, err := q.Query(ctx, selectEvents+`
		WHERE (txid, position) > ($1::text::xid8, $2)
		  AND txid < pg_snapshot_xmin(pg_current_snapshot())
		ORDER BY txid, position
		LIMIT $3`, strconv.FormatUint(after.TxID, 10), after.Position, limit)
	if err != nil {
		return nil, fmt.Errorf("eventstore: read all: %w", err)
	}
	return collect(rows)
}

const selectEvents = `
	-- псевдоним обязателен: столбец с именем txid перехватил бы ORDER BY txid,
	-- и события сортировались бы по тексту ("1000" раньше "999")
	SELECT position, txid::text AS txid_text, event_id, stream_id, stream_version, event_type, schema_version,
	       payload, metadata, correlation_id, causation_id, occurred_at
	FROM events`

func collect(rows pgx.Rows) ([]Event, error) {
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Event, error) {
		var (
			e                      Event
			txid                   string
			correlation, causation pgtype.UUID
			payload, metadata      []byte
		)
		if err := row.Scan(&e.Position, &txid, &e.ID, &e.StreamID, &e.Version, &e.Type, &e.SchemaVersion,
			&payload, &metadata, &correlation, &causation, &e.OccurredAt); err != nil {
			return Event{}, err
		}
		var err error
		if e.TxID, err = strconv.ParseUint(txid, 10, 64); err != nil {
			return Event{}, fmt.Errorf("parse txid: %w", err)
		}
		e.Payload, e.Metadata = payload, metadata
		e.CorrelationID, e.CausationID = correlation.Bytes, causation.Bytes // для NULL остаётся uuid.Nil
		e.OccurredAt = e.OccurredAt.UTC()
		return e, nil
	})
	if err != nil {
		return nil, fmt.Errorf("eventstore: scan events: %w", err)
	}
	return events, nil
}

func validate(streamID string, expectedVersion int64, events []NewEvent) error {
	switch {
	case streamID == "":
		return fmt.Errorf("stream id is empty: %w", ErrInvalidInput)
	case expectedVersion < 0:
		return fmt.Errorf("expected version is negative: %w", ErrInvalidInput)
	case len(events) == 0:
		return fmt.Errorf("no events to append: %w", ErrInvalidInput)
	}
	for i, e := range events {
		switch {
		case e.ID == uuid.Nil:
			return fmt.Errorf("event %d: id is empty: %w", i, ErrInvalidInput)
		case e.Type == "":
			return fmt.Errorf("event %d: type is empty: %w", i, ErrInvalidInput)
		case e.SchemaVersion <= 0:
			return fmt.Errorf("event %d: schema version must be positive: %w", i, ErrInvalidInput)
		case !json.Valid(e.Payload):
			return fmt.Errorf("event %d: payload is not valid JSON: %w", i, ErrInvalidInput)
		case len(e.Metadata) > 0 && !json.Valid(e.Metadata):
			return fmt.Errorf("event %d: metadata is not valid JSON: %w", i, ErrInvalidInput)
		case e.OccurredAt.IsZero():
			return fmt.Errorf("event %d: occurred_at is empty: %w", i, ErrInvalidInput)
		}
	}
	return nil
}

func mapInsertError(streamID string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch pgErr.ConstraintName {
		case streamVersionConstraint:
			return fmt.Errorf("stream %q: concurrent append: %w", streamID, ErrVersionConflict)
		case eventIDConstraint:
			return fmt.Errorf("stream %q: %w", streamID, ErrDuplicateEvent)
		}
	}
	return fmt.Errorf("eventstore: append to %q: %w", streamID, err)
}

func metadataOrEmpty(m json.RawMessage) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}
	return m
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
