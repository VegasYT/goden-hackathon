//go:build integration

package notifications_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/notifications"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/observability/metricstest"
	"goden/backend/internal/platform/postgres"
	"goden/backend/internal/platform/postgres/pgtest"
)

type fakeSender struct {
	calls []string
	chats []int64 // чат каждого вызова, в том же порядке
	// undeliverable - тексты, которые Telegram отклоняет окончательно: чат недоступен
	undeliverable map[string]bool
	fail          map[string]int
	// before вызывается перед ответом: им тест имитирует действия другого экземпляра worker
	before func(text string) error
}

func (s *fakeSender) Send(_ context.Context, chatID int64, text string) error {
	s.calls = append(s.calls, text)
	s.chats = append(s.chats, chatID)
	if s.before != nil {
		if err := s.before(text); err != nil {
			return err
		}
	}
	if s.undeliverable[text] {
		return &notifications.UndeliverableError{}
	}
	if s.fail[text] > 0 {
		s.fail[text]--
		return errors.New("temporary Telegram failure")
	}
	return nil
}

func TestDispatcherRetriesAndContinuesWithNextDelivery(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	queue := notifications.NewQueue(pool)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if err := queue.Enqueue(ctx, tx, notifications.Delivery{
			SourceKey: "a-fails", Kind: notifications.KindExpiryWarning, Body: "first",
		}, now); err != nil {
			return err
		}
		return queue.Enqueue(ctx, tx, notifications.Delivery{
			SourceKey: "b-succeeds", Kind: notifications.KindBatchExpired, Body: "second",
		}, now)
	}))

	sender := &fakeSender{fail: map[string]int{"first": 1}}
	registry := prometheus.NewRegistry()
	dispatcher := notifications.NewDispatcher(queue, sender, clk, slog.New(slog.DiscardHandler), registry)
	dispatcher.SendInterval = 0 // тест о повторах, пауза между отправками в чат здесь мешает

	worked, err := dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	worked, err = dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.True(t, worked, "failed first delivery must not block the second")
	require.Equal(t, []string{"first", "second"}, sender.calls)

	var lastError string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_error FROM notification_deliveries WHERE source_key = 'a-fails'`).Scan(&lastError))
	require.Equal(t, "temporary Telegram failure", lastError, "the cause of the failure is kept for monitoring")

	clk.Advance(time.Second)
	worked, err = dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	require.Equal(t, []string{"first", "second", "first"}, sender.calls)
	worked, err = dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.False(t, worked)
	require.Equal(t, []string{"first", "second", "first"}, sender.calls, "sent deliveries are not sent again")

	require.Equal(t, float64(1), metricstest.Value(t, registry,
		"goden_notification_delivery_attempts_total", "kind", "expiry_warning", "outcome", "error"))
	require.Equal(t, float64(1), metricstest.Value(t, registry,
		"goden_notification_delivery_attempts_total", "kind", "expiry_warning", "outcome", "sent"))
	require.Equal(t, float64(1), metricstest.Value(t, registry,
		"goden_notification_delivery_attempts_total", "kind", "batch_expired", "outcome", "sent"))
	require.Zero(t, metricstest.Value(t, registry, "goden_notification_deliveries_pending"))
}

func enqueue(t *testing.T, pool *pgxpool.Pool, now time.Time, keys ...string) *notifications.Queue {
	t.Helper()
	ctx := context.Background()
	queue := notifications.NewQueue(pool)
	require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		for _, key := range keys {
			if err := queue.Enqueue(ctx, tx, notifications.Delivery{
				SourceKey: key, Kind: notifications.KindExpiryWarning, Body: key,
			}, now); err != nil {
				return err
			}
		}
		return nil
	}))
	return queue
}

// Telegram отклонил сообщение как слишком частое и назвал 30 секунд: до их истечения
// не повторяется ни это уведомление, ни остальные.
func TestDispatcherHonoursRetryAfter(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	queue := enqueue(t, pool, now, "first", "second")
	limited := true
	sender := &fakeSender{before: func(string) error {
		if limited {
			limited = false
			return &notifications.RateLimitedError{RetryAfter: 30 * time.Second}
		}
		return nil
	}}
	dispatcher := notifications.NewDispatcher(queue, sender, clk, slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	dispatcher.SendInterval = 0 // проверяется только время, названное Telegram

	worked, err := dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)

	for _, wait := range []time.Duration{0, 29 * time.Second} {
		clk.Advance(wait)
		worked, err = dispatcher.DispatchOne(ctx)
		require.NoError(t, err)
		require.False(t, worked, "nothing is sent while Telegram asks to wait")
	}
	require.Equal(t, []string{"first"}, sender.calls)

	clk.Advance(time.Second)
	for range 2 {
		worked, err = dispatcher.DispatchOne(ctx)
		require.NoError(t, err)
		require.True(t, worked)
	}
	require.ElementsMatch(t, []string{"first", "second", "first"}, sender.calls)
}

// Пока экземпляр отправлял сообщение, уведомление отметил другой экземпляр: это запись
// в журнале, а не ошибка задачи, которая остановила бы весь worker.
func TestDispatcherSurvivesDeliveryHandledElsewhere(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	queue := enqueue(t, pool, now, "taken")
	sender := &fakeSender{before: func(string) error {
		_, err := pool.Exec(ctx, `UPDATE notification_deliveries SET status = 'sent', sent_at = $1 WHERE source_key = 'taken'`, now)
		return err
	}}
	dispatcher := notifications.NewDispatcher(queue, sender, clock.NewFake(now), slog.New(slog.DiscardHandler), prometheus.NewRegistry())

	worked, err := dispatcher.DispatchOne(ctx)

	require.NoError(t, err)
	require.True(t, worked)
}

// Уведомление уходит в чат, записанный в очереди при постановке: чат точки или, для нуля, общий.
func TestDispatcherSendsToTheChatOfTheDelivery(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	queue := notifications.NewQueue(pool)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if err := queue.Enqueue(ctx, tx, notifications.Delivery{
			SourceKey: "a-point", Kind: notifications.KindExpiryWarning, Body: "point", ChatID: -1001,
		}, now); err != nil {
			return err
		}
		return queue.Enqueue(ctx, tx, notifications.Delivery{
			SourceKey: "b-error", Kind: notifications.KindConsumerError, Body: "error",
		}, now)
	}))

	sender := &fakeSender{}
	dispatcher := notifications.NewDispatcher(queue, sender, clock.NewFake(now), slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	dispatcher.SendInterval = 0
	for range 2 {
		worked, err := dispatcher.DispatchOne(ctx)
		require.NoError(t, err)
		require.True(t, worked)
	}
	require.Equal(t, []string{"point", "error"}, sender.calls)
	require.Equal(t, []int64{-1001, 0}, sender.chats)
}

func statusOf(t *testing.T, pool *pgxpool.Pool, key string) (status, lastError string) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT status, last_error FROM notification_deliveries WHERE source_key = $1`, key).Scan(&status, &lastError))
	return status, lastError
}

// Чат точки задаёт пользователь. Если Telegram отклоняет его окончательно, уведомление не
// повторяется: иначе записи с неверным чатом копились бы и задерживали остальные точки.
func TestDispatcherGivesUpOnAnUndeliverableChat(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	queue := enqueue(t, pool, now, "a-broken-chat", "b-good-chat")
	sender := &fakeSender{undeliverable: map[string]bool{"a-broken-chat": true}}
	registry := prometheus.NewRegistry()
	dispatcher := notifications.NewDispatcher(queue, sender, clk, slog.New(slog.DiscardHandler), registry)

	for range 2 {
		worked, err := dispatcher.DispatchOne(ctx)
		require.NoError(t, err)
		require.True(t, worked)
	}
	status, _ := statusOf(t, pool, "a-broken-chat")
	require.Equal(t, "failed", status)
	status, _ = statusOf(t, pool, "b-good-chat")
	require.Equal(t, "sent", status, "the next point is not held back")

	// повторов нет и ждать больше нечего
	clk.Advance(time.Hour)
	worked, err := dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.False(t, worked)
	require.Equal(t, []string{"a-broken-chat", "b-good-chat"}, sender.calls)
	pending, err := queue.Pending(ctx)
	require.NoError(t, err)
	require.Zero(t, pending, "an undeliverable notification is not waiting for anything")
	require.Equal(t, 1.0, metricstest.Value(t, registry,
		"goden_notification_delivery_attempts_total", "kind", "expiry_warning", "outcome", "failed"))
}

// Временная ошибка повторяется, пока уведомление не устарело: через час оно уже неактуально.
func TestDispatcherStopsRetryingAStaleNotification(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	queue := enqueue(t, pool, now, "outage")
	sender := &fakeSender{fail: map[string]int{"outage": 100}}
	dispatcher := notifications.NewDispatcher(queue, sender, clk, slog.New(slog.DiscardHandler), prometheus.NewRegistry())

	// сбой сети: уведомление остаётся в очереди и повторяется
	for range 3 {
		worked, err := dispatcher.DispatchOne(ctx)
		require.NoError(t, err)
		require.True(t, worked)
		clk.Advance(10 * time.Minute)
	}
	status, lastError := statusOf(t, pool, "outage")
	require.Equal(t, "pending", status)
	require.NotEmpty(t, lastError)

	// прошло больше часа с постановки в очередь
	clk.Advance(time.Hour)
	worked, err := dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	status, lastError = statusOf(t, pool, "outage")
	require.Equal(t, "failed", status)
	require.Contains(t, lastError, "temporary Telegram failure")
	worked, err = dispatcher.DispatchOne(ctx)
	require.NoError(t, err)
	require.False(t, worked)
}

// Ограничение Telegram действует на чат: пауза после отправки и просьба подождать
// задерживают только свой чат, уведомления остальных точек уходят сразу.
func TestDispatcherPacesEveryChatSeparately(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	queue := notifications.NewQueue(pool)
	require.NoError(t, postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		for i, d := range []notifications.Delivery{
			{SourceKey: "a1", ChatID: -1001},
			{SourceKey: "a2", ChatID: -1001},
			{SourceKey: "b1", ChatID: -1002},
			{SourceKey: "c1", ChatID: -1003},
		} {
			d.Kind, d.Body = notifications.KindExpiryWarning, d.SourceKey
			if err := queue.Enqueue(ctx, tx, d, now.Add(time.Duration(i)*time.Millisecond)); err != nil {
				return err
			}
		}
		return nil
	}))
	sender := &fakeSender{before: func(text string) error {
		if text == "c1" {
			return &notifications.RateLimitedError{RetryAfter: time.Minute}
		}
		return nil
	}}
	dispatcher := notifications.NewDispatcher(queue, sender, clk, slog.New(slog.DiscardHandler), prometheus.NewRegistry())
	drain := func() {
		for {
			worked, err := dispatcher.DispatchOne(ctx)
			require.NoError(t, err)
			if !worked {
				return
			}
		}
	}

	// без хода часов: по одному сообщению в каждый чат, второе в первый чат ждёт паузу
	clk.Advance(time.Second)
	drain()
	require.Equal(t, []string{"a1", "b1", "c1"}, sender.calls)

	// пауза первого чата истекла, третий чат Telegram просил не трогать минуту
	clk.Advance(dispatcher.SendInterval)
	drain()
	require.Equal(t, []string{"a1", "b1", "c1", "a2"}, sender.calls)
}
