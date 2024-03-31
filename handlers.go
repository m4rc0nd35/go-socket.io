package socketio

import (
	"sync"
)

type Handlers struct {
	handlers map[string]*Handler
	mu       sync.RWMutex
}

func NewHandlers() *Handlers {
	return &Handlers{
		handlers: make(map[string]*Handler),
	}
}

func (h *Handlers) Set(namespace string, handler *Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers[namespace] = handler
}

func (h *Handlers) Get(nsp string) (*Handler, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	handler, ok := h.handlers[nsp]
	return handler, ok
}
