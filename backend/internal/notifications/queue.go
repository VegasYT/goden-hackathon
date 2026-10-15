// Package notifications доставляет прикладные уведомления из Kafka в Telegram.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Kind - закрытый набор видов Telegram-уведомлений.
type Kind string

const (
	KindExpiryWarning Kind = "expiry_warning"
	// KindBatchExpired - партия просрочена и ждёт ручного списания. Прежний вид expired_write_off
	// (автосписание) больше не создаётся, его записи остаются в таблице.
	KindBatchExpired  Kind = "batch_expired"
	KindConsumerError Kind = "consumer_error"
)

// Delivery - сохранённая попытка доставить одно исходное сообщение.
type Delivery struct {
	SourceKey string
	Kind      Kind
	Body      string
	// ChatID - чат точки, которой адресовано уведомление; ноль - общий чат окружения
	// (только для сообщений об ошибках обработчиков).
	ChatID     int64
	Attempts   int
	LeaseUntil time.Time
	// CreatedAt - когда уведомление поставлено в очередь: от него считается, не устарело ли оно.
	CreatedAt time.Time
}

// Queue хранит уведомления до подтверждённой отправки.
type Queue struct{ pool *pgxpool.Pool }

func NewQueue(pool *pgxpool.Pool) *Queue { return &Queue{pool: pool} }

// Enqueue добавляет уведомление в транзакции Kafka inbox. Повтор источника безопасен.
func (q *Queue) Enqueue(ctx context.Context, tx pgx.Tx, delivery Delivery, now time.Time) error {
	if delivery.SourceKey == "" || delivery.Body == "" || !validKind(delivery.Kind) {
		return errors.New("notification queue: invalid delivery")
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_deliveries
			(source_key, kind, body, chat_id, next_attempt_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (source_key) DO NOTHING`,
		delivery.SourceKey, delivery.Kind, delivery.Body, delivery.ChatID, now.UTC())
	if err != nil {
		return fmt.Errorf("notification queue: enqueue: %w", err)
	}
	return nil
}

// Claim атомарно захватывает следующую готовую запись. Истёкший lease можно забрать снова.
// Записи для чатов из busy пропускаются: в эти чаты сейчас слать нельзя, а остальные ждать не должны.
func (q *Queue) Claim(ctx context.Context, now time.Time, lease time.Duration, busy ...int64) (Delivery, bool, error) {
	if q == nil || q.pool == nil || lease <= 0 {
		return Delivery{}, false, errors.New("notification queue: invalid configuration")
	}
	var delivery Delivery
	err := q.pool.QueryRow(ctx, `
		WITH candidate AS (
			SELECT source_key
			FROM notification_deliveries
			WHERE ((status = 'pending' AND next_attempt_at <= $1)
			   OR (status = 'processing' AND lease_until <= $1))
			  AND chat_id <> ALL($3::bigint[])
			ORDER BY next_attempt_at, created_at, source_key
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE notification_deliveries AS delivery
		SET status = 'processing', attempts = delivery.attempts + 1,
			lease_until = $2, last_error = ''
		FROM candidate
		WHERE delivery.source_key = candidate.source_key
		RETURNING delivery.source_key, delivery.kind, delivery.body, delivery.chat_id,
			delivery.attempts, delivery.lease_until, delivery.created_at`,
		now.UTC(), now.UTC().Add(lease), append([]int64{}, busy...)).Scan(
		&delivery.SourceKey, &delivery.Kind, &delivery.Body, &delivery.ChatID,
		&delivery.Attempts, &delivery.LeaseUntil, &delivery.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Delivery{}, false, nil
	}
	if err != nil {
		return Delivery{}, false, fmt.Errorf("notification queue: claim: %w", err)
	}
	return delivery, true, nil
}

// ErrNotProcessing - уведомление уже не в обработке: его отметил или забрал другой экземпляр worker.
var ErrNotProcessing = errors.New("notification queue: delivery is not processing")

func (q *Queue) MarkSent(ctx context.Context, sourceKey string, now time.Time) error {
	tag, err := q.pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = 'sent', sent_at = $2, lease_until = NULL, last_error = ''
		WHERE source_key = $1 AND status = 'processing'`, sourceKey, now.UTC())
	if err != nil {
		return fmt.Errorf("notification queue: mark sent: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("mark sent %q: %w", sourceKey, ErrNotProcessing)
	}
	return nil
}

func (q *Queue) Retry(ctx context.Context, sourceKey string, next time.Time, cause string) error {
	tag, err := q.pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = 'pending', next_attempt_at = $2, lease_until = NULL,
			last_error = left($3, 1000)
		WHERE source_key = $1 AND status = 'processing'`, sourceKey, next.UTC(), cause)
	if err != nil {
		return fmt.Errorf("notification queue: retry: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("retry %q: %w", sourceKey, ErrNotProcessing)
	}
	return nil
}

// Fail переводит уведомление в конечное состояние "не доставлено": повторно оно не отправляется.
// cause остаётся в записи: по ней видно, что чат точки недоступен.
func (q *Queue) Fail(ctx context.Context, sourceKey, cause string) error {
	tag, err := q.pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = 'failed', lease_until = NULL, last_error = left($2, 1000)
		WHERE source_key = $1 AND status = 'processing'`, sourceKey, cause)
	if err != nil {
		return fmt.Errorf("notification queue: fail: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("fail %q: %w", sourceKey, ErrNotProcessing)
	}
	return nil
}

// Prune удаляет отправленные и недоставленные уведомления, созданные раньше before:
// они нужны только для разбора недавних сбоев, а таблица иначе росла бы без предела.
func (q *Queue) Prune(ctx context.Context, before time.Time) (int64, error) {
	tag, err := q.pool.Exec(ctx, `
		DELETE FROM notification_deliveries WHERE status IN ('sent', 'failed') AND created_at < $1`, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("notification queue: prune: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Pending считает уведомления, которые ещё будут отправляться. Недоставленные сюда не входят.
func (q *Queue) Pending(ctx context.Context) (int64, error) {
	var count int64
	if err := q.pool.QueryRow(ctx, `
		SELECT count(*) FROM notification_deliveries WHERE status IN ('pending', 'processing')`).Scan(&count); err != nil {
		return 0, fmt.Errorf("notification queue: count pending: %w", err)
	}
	return count, nil
}

func validKind(kind Kind) bool {
	return kind == KindExpiryWarning || kind == KindBatchExpired || kind == KindConsumerError
}
