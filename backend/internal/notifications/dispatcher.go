package notifications

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/observability"
)

const (
	defaultDispatchInterval = time.Second
	defaultLeaseDuration    = 30 * time.Second
	defaultRetryMin         = time.Second
	defaultRetryMax         = 5 * time.Minute
	// defaultMaxAge - сколько уведомление повторяется при временных ошибках. Столько же живёт
	// событие до постановки в очередь (maxEventAge): более старое уведомление уже неактуально.
	defaultMaxAge = time.Hour
	// defaultRetention - сколько хранятся завершённые записи очереди. Повторную постановку
	// старого события исключают inbox обработчика и отсечка событий старше часа.
	defaultRetention = 7 * 24 * time.Hour
	pruneInterval    = time.Hour
	// defaultSendInterval - пауза после отправки. Telegram ограничивает группу примерно
	// двадцатью сообщениями в минуту: без паузы пачка уведомлений получала отказы.
	defaultSendInterval = 3 * time.Second
)

// Dispatcher отправляет сохранённые уведомления, не удерживая транзакцию во время сети.
type Dispatcher struct {
	queue  *Queue
	sender Sender
	clock  clock.Clock
	logger *slog.Logger

	PollInterval  time.Duration
	LeaseDuration time.Duration
	MinBackoff    time.Duration
	MaxBackoff    time.Duration
	// SendInterval - пауза после успешной отправки.
	SendInterval time.Duration
	// Retention - сколько хранятся отправленные и недоставленные записи.
	Retention time.Duration
	lastPrune time.Time
	// MaxAge - после этого возраста уведомление, которое не удаётся отправить, считается недоставленным.
	MaxAge time.Duration

	// busyUntil - чаты, в которые сейчас слать нельзя, и момент, когда снова можно: пауза после
	// отправки и время, названное Telegram. Ограничение Telegram действует на чат, поэтому
	// остальные чаты не ждут. Состояние в памяти: после перезапуска паузы начинаются заново.
	busyUntil map[int64]time.Time

	attempts *prometheus.CounterVec
	pending  prometheus.Gauge
}

func NewDispatcher(queue *Queue, sender Sender, clk clock.Clock, logger *slog.Logger,
	registry prometheus.Registerer,
) *Dispatcher {
	attempts := observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: observability.MetricsNamespace, Name: "notification_delivery_attempts_total",
		Help: "Telegram notification delivery attempts by kind and outcome.",
	}, []string{"kind", "outcome"}))
	for _, kind := range []Kind{KindExpiryWarning, KindBatchExpired, KindConsumerError} {
		for _, outcome := range []string{"sent", "error", "failed"} {
			attempts.WithLabelValues(string(kind), outcome)
		}
	}
	return &Dispatcher{
		queue: queue, sender: sender, clock: clk, logger: logger,
		PollInterval: defaultDispatchInterval, LeaseDuration: defaultLeaseDuration,
		MaxAge:     defaultMaxAge,
		Retention:  defaultRetention,
		MinBackoff: defaultRetryMin, MaxBackoff: defaultRetryMax, SendInterval: defaultSendInterval,
		attempts: attempts,
		pending: observability.Register(registry, prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: observability.MetricsNamespace, Name: "notification_deliveries_pending",
			Help: "Notifications waiting for successful Telegram delivery.",
		})),
	}
}

func (d *Dispatcher) Run(ctx context.Context) error {
	if err := d.validate(); err != nil {
		return err
	}
	for {
		didWork, err := d.DispatchOne(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		// После отправки следующая попытка идёт сразу: паузу выдерживает каждый чат отдельно,
		// и запись для другого чата ждать не должна. Нечего отправлять - пауза опроса.
		if !didWork {
			d.prune(ctx)
			if !waitFor(ctx, d.PollInterval) {
				return nil
			}
		}
	}
}

// DispatchOne делает одну попытку и полезен для детерминированных тестов.
func (d *Dispatcher) DispatchOne(ctx context.Context) (bool, error) {
	if err := d.validate(); err != nil {
		return false, err
	}
	now := d.clock.Now()
	delivery, ok, err := d.queue.Claim(ctx, now, d.LeaseDuration, d.busyChats(now)...)
	if err != nil || !ok {
		d.observePending(ctx)
		return false, err
	}
	if err := d.sender.Send(ctx, delivery.ChatID, delivery.Body); err != nil {
		// Окончательный отказ или устаревшее уведомление - конечное состояние, а не повтор:
		// иначе записи с неверным чатом точки копились бы и занимали диспетчер остальных точек.
		var undeliverable *UndeliverableError
		if errors.As(err, &undeliverable) || d.clock.Now().Sub(delivery.CreatedAt) > d.MaxAge {
			if failErr := d.queue.Fail(ctx, delivery.SourceKey, err.Error()); failErr != nil {
				return d.afterMark(ctx, delivery, failErr)
			}
			d.attempts.WithLabelValues(string(delivery.Kind), "failed").Inc()
			d.logger.ErrorContext(ctx, "Telegram notification is undeliverable",
				"kind", delivery.Kind, "source_key", delivery.SourceKey, "error", err)
			d.observePending(ctx)
			return true, nil
		}
		retryIn := d.retryDelay(delivery.Attempts)
		// Telegram назвал время ожидания: раньше него не повторяем и в этот чат больше не шлём
		var limited *RateLimitedError
		if errors.As(err, &limited) {
			retryIn = max(retryIn, limited.RetryAfter)
			d.hold(delivery.ChatID, limited.RetryAfter)
		}
		// причина сохраняется: без неё неверный токен или чат неотличимы от сбоя сети;
		// время считается от момента после отправки, а не до неё: отправка могла длиться долго
		if retryErr := d.queue.Retry(ctx, delivery.SourceKey, d.clock.Now().Add(retryIn), err.Error()); retryErr != nil {
			return d.afterMark(ctx, delivery, retryErr)
		}
		d.attempts.WithLabelValues(string(delivery.Kind), "error").Inc()
		d.logger.ErrorContext(ctx, "Telegram notification delivery failed",
			"kind", delivery.Kind, "source_key", delivery.SourceKey, "retry_in", retryIn.String(), "error", err)
		d.observePending(ctx)
		return true, nil
	}
	d.hold(delivery.ChatID, d.SendInterval)
	if err := d.queue.MarkSent(ctx, delivery.SourceKey, d.clock.Now()); err != nil {
		return d.afterMark(ctx, delivery, err)
	}
	d.attempts.WithLabelValues(string(delivery.Kind), "sent").Inc()
	d.observePending(ctx)
	return true, nil
}

// prune раз в час удаляет завершённые записи старше Retention. Сбой очистки отправке не мешает.
func (d *Dispatcher) prune(ctx context.Context) {
	now := d.clock.Now()
	if d.Retention <= 0 || now.Sub(d.lastPrune) < pruneInterval {
		return
	}
	d.lastPrune = now
	if _, err := d.queue.Prune(ctx, now.Add(-d.Retention)); err != nil {
		d.logger.ErrorContext(ctx, "Notification queue cleanup failed", "error", err)
	}
}

// hold запрещает отправку в чат на время pause.
func (d *Dispatcher) hold(chatID int64, pause time.Duration) {
	if pause <= 0 {
		return
	}
	if d.busyUntil == nil {
		d.busyUntil = map[int64]time.Time{}
	}
	d.busyUntil[chatID] = d.clock.Now().Add(pause)
}

// busyChats возвращает чаты, пауза которых ещё не истекла, и забывает остальные.
func (d *Dispatcher) busyChats(now time.Time) []int64 {
	var busy []int64
	for chatID, until := range d.busyUntil {
		if now.Before(until) {
			busy = append(busy, chatID)
		} else {
			delete(d.busyUntil, chatID)
		}
	}
	return busy
}

// afterMark разбирает ошибку отметки результата. Уведомление, которое уже отметил или забрал
// другой экземпляр, - не сбой: вернувшаяся ошибка остановила бы весь worker.
func (d *Dispatcher) afterMark(ctx context.Context, delivery Delivery, err error) (bool, error) {
	if errors.Is(err, ErrNotProcessing) {
		d.logger.WarnContext(ctx, "Notification was already handled by another worker",
			"kind", delivery.Kind, "source_key", delivery.SourceKey)
		return true, nil
	}
	return true, err
}

func (d *Dispatcher) validate() error {
	if d == nil || d.queue == nil || d.sender == nil || d.clock == nil || d.logger == nil ||
		d.PollInterval <= 0 || d.LeaseDuration <= 0 || d.MinBackoff <= 0 || d.MaxBackoff < d.MinBackoff || d.MaxAge <= 0 {
		return errors.New("notification dispatcher: invalid configuration")
	}
	return nil
}

func (d *Dispatcher) retryDelay(attempt int) time.Duration {
	delay := d.MinBackoff
	for i := 1; i < attempt && delay < d.MaxBackoff; i++ {
		delay = min(delay*2, d.MaxBackoff)
	}
	return delay
}

func (d *Dispatcher) observePending(ctx context.Context) {
	count, err := d.queue.Pending(ctx)
	if err != nil {
		d.logger.ErrorContext(ctx, "Notification queue pending count failed")
		return
	}
	d.pending.Set(float64(count))
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
