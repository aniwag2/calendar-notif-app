package realtime

import (
	"fmt"
	"net/http"
	"sync"
)

// Hub fans out server-sent events to connected clients, keyed by user id.
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[chan string]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[int64]map[chan string]struct{})}
}

func (h *Hub) add(userID int64) chan string {
	ch := make(chan string, 8)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan string]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	return ch
}

func (h *Hub) remove(userID int64, ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[userID]; ok {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.clients, userID)
		}
	}
	close(ch)
}

// Notify pushes an SSE payload (already HTML/text) to all of a user's connections.
func (h *Hub) Notify(userID int64, event, data string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	for ch := range h.clients[userID] {
		select {
		case ch <- msg:
		default: // drop if the client is slow rather than block
		}
	}
}

// ServeHTTP streams events for the given user id over an SSE connection.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request, userID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.add(userID)
	defer h.remove(userID, ch)

	// Initial comment so the connection opens promptly.
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			fmt.Fprint(w, msg)
			flusher.Flush()
		}
	}
}
