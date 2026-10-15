package realtime

import (
	"context"
	"sync"
)

const defaultQueueSize = 16

// Hub неблокирующе раздаёт обновления локальным подписчикам, которым они адресованы.
type Hub struct {
	mu        sync.Mutex
	queueSize int
	nextID    uint64
	clients   map[uint64]client
	closed    bool
}

type client struct {
	scope   Scope
	updates chan Update
}

// NewHub создаёт hub с очередью по умолчанию.
func NewHub() *Hub { return newHub(defaultQueueSize) }

func newHub(queueSize int) *Hub {
	return &Hub{queueSize: queueSize, clients: make(map[uint64]client)}
}

// Subscribe подписывает клиента на сигналы точки и каталога её компании до отмены его контекста.
func (h *Hub) Subscribe(ctx context.Context, scope Scope) <-chan Update {
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	updates := make(chan Update, h.queueSize)
	if h.closed {
		h.mu.Unlock()
		close(updates) // сервер останавливается: поток завершается сразу
		return updates
	}
	h.clients[id] = client{scope: scope, updates: updates}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.remove(id)
	}()
	return updates
}

// Publish отправляет обновление клиентам, которым оно адресовано. Переполненная подписка закрывается.
func (h *Hub) Publish(update Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, c := range h.clients {
		if !c.scope.receives(update) {
			continue
		}
		select {
		case c.updates <- update:
		default:
			delete(h.clients, id)
			close(c.updates)
		}
	}
}

// Close закрывает все подписки и отклоняет новые. Вызывается при остановке сервера:
// http.Server.Shutdown ждёт, пока соединения закроются сами, а поток сам не закрывается.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, c := range h.clients {
		delete(h.clients, id)
		close(c.updates)
	}
}

func (h *Hub) remove(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[id]
	if !ok {
		return
	}
	delete(h.clients, id)
	close(c.updates)
}
