package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"
)

const (
	defaultKeepAlive = 15 * time.Second
	defaultRetry     = 3 * time.Second
)

type scopeKey struct{}

// WithScope кладёт в контекст запроса точку, на которую подписывается клиент. Точку и право
// клиента на неё определяет слой API: обработчик потока о правах не знает.
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

// Handler обслуживает один SSE-поток поверх общего hub.
type Handler struct {
	hub       *Hub
	keepAlive time.Duration
	retry     time.Duration
	// merge - окно, за которое сигналы склеиваются в один; ноль отключает склейку.
	merge time.Duration
}

// defaultMerge - окно склейки сигналов. Заказ на пять ингредиентов - пять складских событий
// подряд; клиенту достаточно перечитать данные один раз, а задержка в четверть секунды незаметна.
const defaultMerge = 250 * time.Millisecond

// NewHandler создаёт SSE-handler с рабочими интервалами.
func NewHandler(hub *Hub) http.Handler {
	return newHandler(hub, defaultKeepAlive, defaultRetry)
}

func newHandler(hub *Hub, keepAlive, retry time.Duration) http.Handler {
	return &Handler{hub: hub, keepAlive: keepAlive, retry: retry, merge: defaultMerge}
}

// writeTimeout - срок на запись одного события. Без него клиент, который держит соединение
// и не читает, удерживал бы обработчик и сокет бесконечно.
const writeTimeout = 10 * time.Second

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	controller := http.NewResponseController(w)
	// срок продлевается перед каждой записью; ошибка означает, что соединение его не поддерживает
	// срок сокета отсчитывается от настоящих часов, а не от clock.Clock: это не время предметной области
	deadline := func() { _ = controller.SetWriteDeadline(time.Now().Add(writeTimeout)) } //nolint:forbidigo // срок записи в сокет
	deadline()
	// без точки в контексте подписка пуста: клиент получит sync и keepalive, но ни одного сигнала
	scope, _ := r.Context().Value(scopeKey{}).(Scope)
	updates := h.hub.Subscribe(r.Context(), scope)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "retry: %d\n", h.retry.Milliseconds()); err != nil {
		return
	}
	if err := writeEvent(w, "", "sync", struct {
		Categories []Category `json:"categories"`
	}{Categories: allCategories}); err != nil {
		return
	}
	if err := controller.Flush(); err != nil {
		return
	}

	keepAlive := time.NewTicker(h.keepAlive)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case update, open := <-updates:
			if !open {
				return
			}
			update, open = h.collect(r.Context(), updates, update)
			deadline()
			if err := writeEvent(w, update.ID.String(), "update", update); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
			if !open {
				return
			}
		case <-keepAlive.C:
			deadline()
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

// collect ждёт окно склейки и объединяет пришедшие за него сигналы в один: идентификатор и время
// последнего события, категории всех. open = false - подписка закрыта, после записи поток завершается.
func (h *Handler) collect(ctx context.Context, updates <-chan Update, first Update) (merged Update, open bool) {
	merged = first
	if h.merge <= 0 {
		return merged, true
	}
	timer := time.NewTimer(h.merge)
	defer timer.Stop()
	for {
		select {
		case next, open := <-updates:
			if !open {
				return merged, false
			}
			merged = mergeUpdates(merged, next)
		case <-timer.C:
			return merged, true
		case <-ctx.Done():
			return merged, false
		}
	}
}

func mergeUpdates(a, b Update) Update {
	var categories []Category
	for _, category := range allCategories {
		if slices.Contains(a.Categories, category) || slices.Contains(b.Categories, category) {
			categories = append(categories, category)
		}
	}
	b.Categories = categories
	return b
}

func writeEvent(w http.ResponseWriter, id, event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("realtime: encode %s: %w", event, err)
	}
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
	return err
}
