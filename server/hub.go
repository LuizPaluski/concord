package main

import (
	"encoding/json"
	"sort"
	"sync"
)

type client struct {
	user  user
	voice int64 // canal de voz em que está, 0 se nenhum
	send  chan []byte
}

type hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

type presence struct {
	Name  string `json:"name"`
	Voice int64  `json:"voice"`
}

func newHub() *hub {
	return &hub{clients: map[*client]struct{}{}}
}

func (h *hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	h.broadcastPresenceLocked()
}

func (h *hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dropLocked(c) {
		h.broadcastPresenceLocked()
	}
}

func (h *hub) setVoice(c *client, channel int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	c.voice = channel
	h.broadcastPresenceLocked()
}

func (h *hub) broadcast(event any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(event)
}

func (h *hub) broadcastLocked(event any) {
	b, _ := json.Marshal(event)
	dropped := false
	for c := range h.clients {
		select {
		case c.send <- b:
		default:
			// Cliente que não esvazia o buffer é derrubado; ele reconecta e recarrega.
			dropped = h.dropLocked(c) || dropped
		}
	}
	if dropped {
		h.broadcastPresenceLocked()
	}
}

func (h *hub) dropLocked(c *client) bool {
	if _, ok := h.clients[c]; !ok {
		return false
	}
	delete(h.clients, c)
	close(c.send)
	return true
}

func (h *hub) broadcastPresenceLocked() {
	voice := map[string]int64{}
	for c := range h.clients {
		if c.voice != 0 || voice[c.user.Name] == 0 {
			voice[c.user.Name] = c.voice
		}
	}
	users := make([]presence, 0, len(voice))
	for name, ch := range voice {
		users = append(users, presence{name, ch})
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	h.broadcastLocked(map[string]any{"type": "presence", "users": users})
}
