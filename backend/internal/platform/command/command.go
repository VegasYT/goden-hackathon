// Package command выполняет команды записи поверх Event Store (ADR-002).
//
// Доменный модуль описывает команду в Spec и передаёт функцию с бизнес-логикой.
// Пакет отвечает за транзакцию, блокировки по потокам, идемпотентность,
// повтор при конфликте и служебные поля событий.
package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
)

var (
	// ErrKeyReused - ключ идемпотентности уже использован командой с другим содержимым запроса.
	ErrKeyReused = errors.New("command: idempotency key reused with a different request")
	// ErrInvalidSpec - некорректное описание команды, до базы вызов не доходит.
	ErrInvalidSpec = errors.New("command: invalid spec")
)

// maxAttempts ограничивает повторы при конфликте версий и взаимной блокировке.
const maxAttempts = 3

// Spec описывает команду.
type Spec struct {
	// Name - имя команды, например "RegisterDelivery". Попадает в метаданные событий.
	Name  string
	Actor eventstore.Actor
	// Locks - потоки, по которым команде нужна очередь. Все складские потоки,
	// в которые команда пишет, объявляются здесь: inventory.StockStream(pointID, productID).
	Locks []string
	// IdempotencyKey - ключ запроса. С пустым ключом команда выполняется при каждом вызове.
	IdempotencyKey string
	// Request - содержимое запроса. По нему повтор отличается от другого запроса с тем же ключом.
	Request any
}

// Runner выполняет команды.
type Runner struct {
	pool   *pgxpool.Pool
	clock  clock.Clock
	logger *slog.Logger

	registry *prometheus.Registry
	commands *prometheus.CounterVec
	retries  *prometheus.CounterVec
	events   *prometheus.CounterVec
}

func NewRunner(pool *pgxpool.Pool, clk clock.Clock, logger *slog.Logger) *Runner {
	r := &Runner{
		pool: pool, clock: clk, logger: logger,
		registry: observability.NewRegistry(),
		commands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "commands_total",
			Help: "Write commands by name and outcome: ok, replayed (stored result for an idempotency key), error, exhausted (gave up after retries).",
		}, []string{"command", "outcome"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "command_retries_total",
			Help: "Command attempts repeated after a stream version conflict or a deadlock.",
		}, []string{"command"}),
		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "events_appended_total",
			Help: "Domain events committed to the Event Store by type.",
		}, []string{"type"}),
	}
	r.registry.MustRegister(r.commands, r.retries, r.events)
	return r
}

// Now возвращает текущее время по часам команд. Им пользуются чтения, которым нужен тот же
// момент проверки, что и командам: просрочена ли партия.
func (r *Runner) Now() time.Time { return r.clock.Now() }

// ActorOf возвращает автора команды, записавшей событие.
func ActorOf(e eventstore.Event) (eventstore.Actor, error) {
	var metadata struct {
		Actor eventstore.Actor `json:"actor"`
	}
	if err := json.Unmarshal(e.Metadata, &metadata); err != nil {
		return eventstore.Actor{}, fmt.Errorf("command: metadata of event %s: %w", e.ID, err)
	}
	return metadata.Actor, nil
}

// Registry возвращает реестр метрик процесса. В нём счётчики команд; модули
// регистрируют здесь свои метрики, а процесс отдаёт его на /metrics.
func (r *Runner) Registry() *prometheus.Registry { return r.registry }

// Tx - транзакция команды. Через встроенный pgx.Tx команда читает и меняет свои таблицы.
type Tx struct {
	pgx.Tx
	now           time.Time
	commandID     uuid.UUID
	correlationID uuid.UUID
	metadata      json.RawMessage
	locks         []string // отсортированы
	events        *prometheus.CounterVec
	onCommit      *[]func() // свой список у каждой попытки
}

// OnCommit откладывает fn до успешной фиксации транзакции команды. Так считают метрики:
// попытка, которая откатилась или повторилась, и ответ из сохранённого результата fn не вызывают.
func (tx *Tx) OnCommit(fn func()) {
	*tx.onCommit = append(*tx.onCommit, fn)
}

// Now возвращает время начала попытки. Оно одно на всю команду и стоит во всех её событиях.
func (tx *Tx) Now() time.Time { return tx.now }

// Locked сообщает, объявила ли команда поток в Spec.Locks.
func (tx *Tx) Locked(streamID string) bool {
	_, found := slices.BinarySearch(tx.locks, streamID)
	return found
}

// Append добавляет доменные события в поток и заполняет их служебные поля.
func (tx *Tx) Append(ctx context.Context, streamID string, expectedVersion int64, payloads ...eventstore.Payload) (int64, error) {
	events := make([]eventstore.NewEvent, len(payloads))
	for i, p := range payloads {
		raw, err := eventstore.Encode(p)
		if err != nil {
			return 0, err
		}
		events[i] = eventstore.NewEvent{
			ID:            uuid.Must(uuid.NewV7()),
			Type:          p.EventType(),
			SchemaVersion: p.SchemaVersion(),
			Payload:       raw,
			Metadata:      tx.metadata,
			CorrelationID: tx.correlationID,
			CausationID:   tx.commandID,
			OccurredAt:    tx.now,
		}
	}
	version, err := eventstore.Append(ctx, tx.Tx, streamID, expectedVersion, events...)
	if err != nil {
		return 0, err
	}
	tx.OnCommit(func() {
		for _, p := range payloads {
			tx.events.WithLabelValues(p.EventType()).Inc()
		}
	})
	return version, nil
}

// Run выполняет команду: fn вызывается в транзакции после взятия блокировок.
// Результат T сохраняется для повторов с тем же ключом, поэтому должен
// сериализоваться в JSON и обратно без потерь.
//
// При конфликте версий fn вызывается заново в новой транзакции, поэтому она
// не должна иметь побочных эффектов вне переданной транзакции.
func Run[T any](ctx context.Context, r *Runner, spec Spec, fn func(ctx context.Context, tx *Tx) (T, error)) (T, error) {
	var zero T
	if err := spec.validate(); err != nil {
		return zero, err
	}

	requestHash, err := hashRequest(spec)
	if err != nil {
		return zero, err
	}
	metadata, err := json.Marshal(struct {
		Actor   eventstore.Actor `json:"actor"`
		Command string           `json:"command"`
	}{spec.Actor, spec.Name})
	if err != nil {
		return zero, fmt.Errorf("command %s: encode metadata: %w", spec.Name, err)
	}
	correlationID, ok := observability.CorrelationID(ctx)
	if !ok {
		correlationID = uuid.Must(uuid.NewV7())
	}

	locks := slices.Clone(spec.Locks)
	slices.Sort(locks)
	locks = slices.Compact(locks)

	// идентификатор команды не меняется между попытками
	tmpl := Tx{
		commandID: uuid.Must(uuid.NewV7()), correlationID: correlationID, metadata: metadata, locks: locks,
		events: r.events,
	}

	for attempt := 1; ; attempt++ {
		result, replayed, err := attemptOnce(ctx, r, spec, requestHash, tmpl, fn)
		switch {
		case err == nil && replayed:
			r.commands.WithLabelValues(spec.Name, "replayed").Inc()
			return result, nil
		case err == nil:
			r.commands.WithLabelValues(spec.Name, "ok").Inc()
			return result, nil
		case !retryable(err):
			r.commands.WithLabelValues(spec.Name, "error").Inc()
			return zero, err
		case attempt == maxAttempts:
			r.commands.WithLabelValues(spec.Name, "exhausted").Inc()
			return zero, fmt.Errorf("command %s: gave up after %d attempts: %w", spec.Name, maxAttempts, err)
		}
		r.retries.WithLabelValues(spec.Name).Inc()
		r.logger.WarnContext(ctx, "command retry", "command", spec.Name, "attempt", attempt, "error", err)
	}
}

// Stored возвращает сохранённый результат команды с ключом и запросом из spec, не выполняя её.
// Им пользуется команда, у которой есть проверки до транзакции: повтор уже выполненного запроса
// обязан вернуть прежний результат, даже если с тех пор проверка перестала проходить.
// Команда, которая ещё выполняется, результата не имеет: found = false, её дождётся Run.
func Stored[T any](ctx context.Context, r *Runner, spec Spec) (result T, found bool, err error) {
	if spec.IdempotencyKey == "" {
		return result, false, nil
	}
	if err := spec.validate(); err != nil {
		return result, false, err
	}
	requestHash, err := hashRequest(spec)
	if err != nil {
		return result, false, err
	}
	var (
		storedHash string
		stored     []byte
	)
	err = r.pool.QueryRow(ctx, `
		SELECT request_hash, result FROM command_results
		WHERE command_name = $1 AND actor = $2 AND idempotency_key = $3`,
		spec.Name, actorKey(spec.Actor), spec.IdempotencyKey).Scan(&storedHash, &stored)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return result, false, nil
	case err != nil:
		return result, false, fmt.Errorf("command %s: read stored result: %w", spec.Name, err)
	case storedHash != requestHash:
		return result, false, fmt.Errorf("command %s: %w", spec.Name, ErrKeyReused)
	case stored == nil:
		return result, false, nil
	}
	if err := json.Unmarshal(stored, &result); err != nil {
		return result, false, fmt.Errorf("command %s: decode stored result: %w", spec.Name, err)
	}
	return result, true, nil
}

func attemptOnce[T any](ctx context.Context, r *Runner, spec Spec, requestHash string, tmpl Tx,
	fn func(ctx context.Context, tx *Tx) (T, error),
) (result T, replayed bool, err error) {
	var onCommit []func()
	err = pgx.BeginFunc(ctx, r.pool, func(pgtx pgx.Tx) error {
		// Блокировки идут до любой записи: запись назначает транзакции txid, а порядок txid
		// должен совпадать с порядком версий потока, иначе читатели получат события не по порядку.
		if err := lock(ctx, pgtx, tmpl.locks); err != nil {
			return err
		}

		tx := tmpl
		tx.Tx, tx.now, tx.onCommit = pgtx, r.clock.Now(), &onCommit

		if spec.IdempotencyKey != "" {
			stored, done, err := claimKey(ctx, &tx, spec, requestHash)
			if err != nil {
				return err
			}
			if done {
				if err := json.Unmarshal(stored, &result); err != nil {
					return fmt.Errorf("command %s: decode stored result: %w", spec.Name, err)
				}
				replayed = true
				return nil
			}
		}

		var err error
		if result, err = fn(ctx, &tx); err != nil {
			return err
		}

		if spec.IdempotencyKey != "" {
			return saveResult(ctx, &tx, spec, result)
		}
		return nil
	})
	if err != nil {
		return result, false, err
	}
	for _, fn := range onCommit {
		fn()
	}
	return result, replayed, nil
}

// lock берёт блокировки в порядке сортировки: единый порядок исключает взаимные блокировки команд.
func lock(ctx context.Context, tx pgx.Tx, streams []string) error {
	if len(streams) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, stream := range streams {
		batch.Queue(`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, stream)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("command: lock streams: %w", err)
	}
	return nil
}

// claimKey занимает ключ идемпотентности. Если ключ уже занят завершённой командой
// с тем же запросом, возвращает её сохранённый результат и done = true.
//
// Одновременный вызов с тем же ключом ждёт здесь завершения первой транзакции.
func claimKey(ctx context.Context, tx *Tx, spec Spec, requestHash string) (stored []byte, done bool, err error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO command_results (command_name, actor, idempotency_key, request_hash, command_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING`,
		spec.Name, actorKey(spec.Actor), spec.IdempotencyKey, requestHash, tx.commandID, tx.now)
	if err != nil {
		return nil, false, fmt.Errorf("command %s: claim idempotency key: %w", spec.Name, err)
	}
	if tag.RowsAffected() == 1 {
		return nil, false, nil
	}

	var storedHash string
	err = tx.QueryRow(ctx, `
		SELECT request_hash, result FROM command_results
		WHERE command_name = $1 AND actor = $2 AND idempotency_key = $3`,
		spec.Name, actorKey(spec.Actor), spec.IdempotencyKey).Scan(&storedHash, &stored)
	if err != nil {
		return nil, false, fmt.Errorf("command %s: read stored result: %w", spec.Name, err)
	}
	if storedHash != requestHash {
		return nil, false, fmt.Errorf("command %s: %w", spec.Name, ErrKeyReused)
	}
	return stored, true, nil
}

// actorKey - автор в ключе результата команды. Ключ идемпотентности принадлежит автору:
// чужой ключ нельзя ни занять, ни обнаружить по отказу.
func actorKey(actor eventstore.Actor) string {
	return string(actor.Type) + ":" + actor.ID
}

func saveResult(ctx context.Context, tx *Tx, spec Spec, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("command %s: encode result: %w", spec.Name, err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE command_results SET result = $4
		WHERE command_name = $1 AND actor = $2 AND idempotency_key = $3`,
		spec.Name, actorKey(spec.Actor), spec.IdempotencyKey, raw)
	if err != nil {
		return fmt.Errorf("command %s: save result: %w", spec.Name, err)
	}
	return nil
}

func (s Spec) validate() error {
	switch {
	case s.Name == "":
		return fmt.Errorf("name is empty: %w", ErrInvalidSpec)
	case s.Actor.ID == "":
		return fmt.Errorf("%s: actor id is empty: %w", s.Name, ErrInvalidSpec)
	case s.Actor.Type != eventstore.ActorUser && s.Actor.Type != eventstore.ActorService:
		return fmt.Errorf("%s: unknown actor type %q: %w", s.Name, s.Actor.Type, ErrInvalidSpec)
	case slices.Contains(s.Locks, ""):
		return fmt.Errorf("%s: empty stream id in locks: %w", s.Name, ErrInvalidSpec)
	}
	return nil
}

func hashRequest(spec Spec) (string, error) {
	if spec.IdempotencyKey == "" {
		return "", nil
	}
	// Автор входит и в ключ результата (actorKey), и в хеш: результат одного автора
	// никогда не возвращается другому.
	raw, err := json.Marshal(struct {
		Actor   eventstore.Actor
		Request any
	}{spec.Actor, spec.Request})
	if err != nil {
		return "", fmt.Errorf("%s: request is not serializable: %w", spec.Name, ErrInvalidSpec)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

const (
	deadlockDetected     = "40P01"
	serializationFailure = "40001"
)

func retryable(err error) bool {
	if errors.Is(err, eventstore.ErrVersionConflict) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == deadlockDetected || pgErr.Code == serializationFailure)
}
