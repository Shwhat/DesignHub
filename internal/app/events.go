package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event tells browsers that a record changed so open views can refresh.
// Events carry identifiers only; the browser fetches the record through the
// API, which applies permissions.
type Event struct {
	Type   string   `json:"type"` // record, notification, board
	ID     string   `json:"id,omitempty"`
	Space  string   `json:"space,omitempty"`
	Status string   `json:"status,omitempty"`
	Users  []string `json:"-"` // when set, only these users receive the event
}

type subscriber struct {
	userID string
	ch     chan Event
}

// Hub fans events out to connected browsers.
type Hub struct {
	mu   sync.Mutex
	subs map[*subscriber]bool
}

// NewHub returns an empty hub.
func NewHub() *Hub { return &Hub{subs: map[*subscriber]bool{}} }

// Publish sends an event to every matching subscriber without blocking.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if len(e.Users) > 0 && !contains(e.Users, s.userID) {
			continue
		}
		select {
		case s.ch <- e:
		default: // slow client; it refreshes on reconnect
		}
	}
}

func (h *Hub) add(userID string) *subscriber {
	s := &subscriber{userID: userID, ch: make(chan Event, 64)}
	h.mu.Lock()
	h.subs[s] = true
	h.mu.Unlock()
	return s
}

func (h *Hub) remove(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// apiEvents streams events to the browser with Server-Sent Events.
func (a *App) apiEvents(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	fl, okf := w.(http.Flusher)
	if !okf {
		return errf(http.StatusInternalServerError, "streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	s := a.Events.add(u.ID)
	defer a.Events.remove(s)
	fmt.Fprint(w, "retry: 3000\n\n")
	fl.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case e := <-s.ch:
			if e.Space != "" && !u.CanIn(e.Space, PermSpaceView) {
				continue
			}
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
