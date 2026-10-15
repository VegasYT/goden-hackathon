//go:build integration

package eventstore_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

// appendTx добавляет события в отдельной зафиксированной транзакции.
func appendTx(t *testing.T, pool *pgxpool.Pool, streamID string, expected int64, events ...eventstore.NewEvent) (int64, error) {
	t.Helper()
	var version int64
	err := postgres.InTx(context.Background(), pool, func(tx pgx.Tx) (err error) {
		version, err = eventstore.Append(context.Background(), tx, streamID, expected, events...)
		return err
	})
	return version, err
}

func mustAppend(t *testing.T, pool *pgxpool.Pool, streamID string, expected int64, n int) {
	t.Helper()
	events := make([]eventstore.NewEvent, n)
	for i := range events {
		events[i] = newEvent("Tested")
	}
	_, err := appendTx(t, pool, streamID, expected, events...)
	require.NoError(t, err)
}

func versions(events []eventstore.Event) []int64 {
	out := make([]int64, len(events))
	for i, e := range events {
		out[i] = e.Version
	}
	return out
}

func ids(events []eventstore.Event) []uuid.UUID {
	out := make([]uuid.UUID, len(events))
	for i, e := range events {
		out[i] = e.ID
	}
	return out
}

func TestAppendNewStream(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)

	version, err := appendTx(t, pool, "s-1", 0, newEvent("Tested"), newEvent("Tested"))

	require.NoError(t, err)
	require.Equal(t, int64(2), version)
	events, err := eventstore.ReadStream(ctx, pool, "s-1", 1)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, versions(events))
}

func TestAppendContinuesStream(t *testing.T) {
	pool := pgtest.NewDB(t)
	mustAppend(t, pool, "s-1", 0, 2)

	version, err := appendTx(t, pool, "s-1", 2, newEvent("Tested"))

	require.NoError(t, err)
	require.Equal(t, int64(3), version)
}

func TestAppendVersionConflict(t *testing.T) {
	tests := []struct {
		name     string
		expected int64
	}{
		{"stale expected version", 2},
		{"expected version ahead of stream", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, pool := context.Background(), pgtest.NewDB(t)
			mustAppend(t, pool, "s-1", 0, 3)

			_, err := appendTx(t, pool, "s-1", tt.expected, newEvent("Tested"))

			require.ErrorIs(t, err, eventstore.ErrVersionConflict)
			version, err := eventstore.StreamVersion(ctx, pool, "s-1")
			require.NoError(t, err)
			require.Equal(t, int64(3), version, "stream must stay unchanged, without gaps")
		})
	}
}

func TestAppendConcurrentSameExpectedVersion(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)

	first, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer first.Rollback(ctx) //nolint:errcheck
	_, err = eventstore.Append(ctx, first, "s-1", 0, newEvent("Tested"))
	require.NoError(t, err)

	// вторая транзакция прошла проверку версии и ждёт на ограничении уникальности
	secondErr := make(chan error, 1)
	go func() {
		_, err := appendTx(t, pool, "s-1", 0, newEvent("Tested"))
		secondErr <- err
	}()

	require.NoError(t, first.Commit(ctx))
	require.ErrorIs(t, <-secondErr, eventstore.ErrVersionConflict)

	version, err := eventstore.StreamVersion(ctx, pool, "s-1")
	require.NoError(t, err)
	require.Equal(t, int64(1), version)
}

func TestAppendMultipleStreamsIsAtomic(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	mustAppend(t, pool, "b", 0, 1)

	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := eventstore.Append(ctx, tx, "a", 0, newEvent("Tested")); err != nil {
			return err
		}
		_, err := eventstore.Append(ctx, tx, "b", 0, newEvent("Tested")) // устаревшая версия
		return err
	})
	require.ErrorIs(t, err, eventstore.ErrVersionConflict)

	for stream, want := range map[string]int64{"a": 0, "b": 1} {
		version, err := eventstore.StreamVersion(ctx, pool, stream)
		require.NoError(t, err)
		require.Equal(t, want, version, "stream %s", stream)
	}

	// успешная запись в два потока и в постороннюю таблицу одной транзакцией
	_, err = pool.Exec(ctx, "CREATE TABLE balances (stream text PRIMARY KEY)")
	require.NoError(t, err)
	err = postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := eventstore.Append(ctx, tx, "a", 0, newEvent("Tested")); err != nil {
			return err
		}
		if _, err := eventstore.Append(ctx, tx, "b", 1, newEvent("Tested")); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "INSERT INTO balances VALUES ('a')")
		return err
	})
	require.NoError(t, err)

	for stream, want := range map[string]int64{"a": 1, "b": 2} {
		version, err := eventstore.StreamVersion(ctx, pool, stream)
		require.NoError(t, err)
		require.Equal(t, want, version, "stream %s", stream)
	}
}

func TestStoredEventMatchesAppended(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	in := newEvent("BatchReceived")
	in.SchemaVersion = 2
	in.Payload = json.RawMessage(`{"quantity":1500,"batch_id":"b-1"}`)
	in.Metadata = json.RawMessage(`{"source":"test"}`)
	in.CorrelationID = uuid.Must(uuid.NewV7())
	in.CausationID = uuid.Must(uuid.NewV7())
	withoutOptional := newEvent("Tested")

	_, err := appendTx(t, pool, "stock-1", 0, in, withoutOptional)
	require.NoError(t, err)

	events, err := eventstore.ReadStream(ctx, pool, "stock-1", 1)
	require.NoError(t, err)
	require.Len(t, events, 2)

	out := events[0]
	require.Equal(t, in.ID, out.ID)
	require.Equal(t, "stock-1", out.StreamID)
	require.Equal(t, int64(1), out.Version)
	require.Equal(t, in.Type, out.Type)
	require.Equal(t, in.SchemaVersion, out.SchemaVersion)
	require.JSONEq(t, string(in.Payload), string(out.Payload))
	require.JSONEq(t, string(in.Metadata), string(out.Metadata))
	require.Equal(t, in.CorrelationID, out.CorrelationID)
	require.Equal(t, in.CausationID, out.CausationID)
	require.Equal(t, occurredAt, out.OccurredAt)
	require.Positive(t, out.Position)
	require.Positive(t, out.TxID)

	require.Equal(t, uuid.Nil, events[1].CorrelationID)
	require.Equal(t, uuid.Nil, events[1].CausationID)
	require.JSONEq(t, `{}`, string(events[1].Metadata))
}

func TestAppendDuplicateEventID(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	event := newEvent("Tested")
	_, err := appendTx(t, pool, "s-1", 0, event)
	require.NoError(t, err)

	_, err = appendTx(t, pool, "s-2", 0, event)

	require.ErrorIs(t, err, eventstore.ErrDuplicateEvent)
	version, err := eventstore.StreamVersion(ctx, pool, "s-2")
	require.NoError(t, err)
	require.Zero(t, version)
}

func TestEventsAreImmutable(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	mustAppend(t, pool, "s-1", 0, 1)

	for _, query := range []string{
		"UPDATE events SET event_type = 'Changed'",
		"DELETE FROM events",
		"TRUNCATE events",
	} {
		_, err := pool.Exec(ctx, query)
		require.ErrorContains(t, err, "append-only", query)
	}

	events, err := eventstore.ReadStream(ctx, pool, "s-1", 1)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "Tested", events[0].Type)
}

func TestReadStream(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	mustAppend(t, pool, "s-1", 0, 3)
	mustAppend(t, pool, "other", 0, 1)

	all, err := eventstore.ReadStream(ctx, pool, "s-1", 1)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, versions(all))

	fromMiddle, err := eventstore.ReadStream(ctx, pool, "s-1", 3)
	require.NoError(t, err)
	require.Equal(t, []int64{3}, versions(fromMiddle))

	missing, err := eventstore.ReadStream(ctx, pool, "never-written", 1)
	require.NoError(t, err)
	require.Empty(t, missing)
	version, err := eventstore.StreamVersion(ctx, pool, "never-written")
	require.NoError(t, err)
	require.Zero(t, version)
}

func TestReadAllInPages(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	mustAppend(t, pool, "a", 0, 2)
	mustAppend(t, pool, "b", 0, 1)
	mustAppend(t, pool, "a", 2, 2)

	var (
		checkpoint eventstore.Checkpoint
		pageSizes  []int
		seen       []uuid.UUID
	)
	for {
		page, err := eventstore.ReadAll(ctx, pool, checkpoint, 2)
		require.NoError(t, err)
		pageSizes = append(pageSizes, len(page))
		if len(page) == 0 {
			break
		}
		seen = append(seen, ids(page)...)
		checkpoint = page[len(page)-1].Checkpoint()
	}

	require.Equal(t, []int{2, 2, 1, 0}, pageSizes)
	require.Len(t, seen, 5)
	unique := map[uuid.UUID]bool{}
	for _, id := range seen {
		unique[id] = true
	}
	require.Len(t, unique, 5, "every event exactly once")
}

// Событие транзакции, начавшейся раньше, получает меньший txid, но может зафиксироваться позже.
// Читатель не должен уйти вперёд и пропустить его.
func TestReadAllDoesNotSkipEventsOfOpenTransaction(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	eventA, eventB := newEvent("A"), newEvent("B")

	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer txA.Rollback(ctx) //nolint:errcheck
	_, err = eventstore.Append(ctx, txA, "a", 0, eventA)
	require.NoError(t, err)

	_, err = appendTx(t, pool, "b", 0, eventB) // B начата позже, а зафиксирована раньше A
	require.NoError(t, err)

	visible, err := eventstore.ReadAll(ctx, pool, eventstore.Checkpoint{}, 10)
	require.NoError(t, err)
	require.Empty(t, visible, "event B must wait for the older open transaction")

	require.NoError(t, txA.Commit(ctx))

	visible, err = eventstore.ReadAll(ctx, pool, eventstore.Checkpoint{}, 10)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{eventA.ID, eventB.ID}, ids(visible))
}

func TestReadAllIgnoresRolledBackTransaction(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	kept := newEvent("Kept")

	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = eventstore.Append(ctx, txA, "a", 0, newEvent("Discarded"))
	require.NoError(t, err)
	require.NoError(t, txA.Rollback(ctx))

	_, err = appendTx(t, pool, "b", 0, kept)
	require.NoError(t, err)

	visible, err := eventstore.ReadAll(ctx, pool, eventstore.Checkpoint{}, 10)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{kept.ID}, ids(visible))
}

// txid растёт как число: событие с txid 1000 идёт после события с txid 999.
// Сортировка по текстовому представлению поменяла бы их местами.
func TestReadAllOrdersTxIDNumerically(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	before, after := newEvent("Before"), newEvent("After")

	_, err := appendTx(t, pool, "a", 0, before)
	require.NoError(t, err)

	// сжигаем номера транзакций до следующего разряда: каждая вложенная транзакция с записью получает свой
	var current int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT pg_current_xact_id()::text::bigint").Scan(&current))
	nextPowerOfTen := int64(10)
	for nextPowerOfTen <= current {
		nextPowerOfTen *= 10
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		DO $$
		BEGIN
			CREATE TEMP TABLE burned (i bigint) ON COMMIT DROP;
			FOR i IN 1..%d LOOP
				BEGIN
					INSERT INTO burned VALUES (i);
				EXCEPTION WHEN division_by_zero THEN NULL;
				END;
			END LOOP;
		END $$`, nextPowerOfTen-current))
	require.NoError(t, err)

	_, err = appendTx(t, pool, "b", 0, after)
	require.NoError(t, err)

	events, err := eventstore.ReadAll(ctx, pool, eventstore.Checkpoint{}, 10)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{before.ID, after.ID}, ids(events))
	require.Less(t, len(strconv.FormatUint(events[0].TxID, 10)), len(strconv.FormatUint(events[1].TxID, 10)),
		"the test must cross a decimal digit boundary")

	// чтение с позиции первого события возвращает только второе
	rest, err := eventstore.ReadAll(ctx, pool, events[0].Checkpoint(), 10)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{after.ID}, ids(rest))
}

func TestReadStreamBackward(t *testing.T) {
	pool := pgtest.NewDB(t)
	ctx := context.Background()
	read := func(before int64, limit int) []int64 {
		t.Helper()
		events, err := eventstore.ReadStreamBackward(ctx, pool, "stock-1", before, limit)
		require.NoError(t, err)
		return versions(events)
	}

	require.Empty(t, read(1, 5), "empty stream")

	mustAppend(t, pool, "stock-1", 0, 10)
	mustAppend(t, pool, "stock-2", 0, 1)

	require.Equal(t, []int64{10, 9, 8}, read(11, 3), "the last events")
	require.Equal(t, []int64{7, 6, 5}, read(8, 3), "the next page")
	require.Equal(t, []int64{2, 1}, read(3, 5), "the start of the stream")
	require.Empty(t, read(1, 5))
	require.Empty(t, read(-4, 5), "an offset beyond the stream")

	_, err := eventstore.ReadStreamBackward(ctx, pool, "stock-1", 11, 0)
	require.ErrorIs(t, err, eventstore.ErrInvalidInput)
}

func TestReadFirst(t *testing.T) {
	pool := pgtest.NewDB(t)
	ctx := context.Background()
	mustAppend(t, pool, "delivery-1", 0, 3)
	mustAppend(t, pool, "delivery-2", 0, 1)
	mustAppend(t, pool, "delivery-3", 0, 2)

	events, err := eventstore.ReadFirst(ctx, pool, []string{"delivery-1", "delivery-3", "delivery-none"})
	require.NoError(t, err)
	streams := make([]string, len(events))
	for i, e := range events {
		streams[i] = e.StreamID
	}
	require.ElementsMatch(t, []string{"delivery-1", "delivery-3"}, streams, "a stream without events is absent")
	require.Equal(t, []int64{1, 1}, versions(events))

	events, err = eventstore.ReadFirst(ctx, nil, nil)
	require.NoError(t, err, "an empty set makes no query")
	require.Empty(t, events)
}
