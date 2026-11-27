package main

import (
	"encoding/json"
	"slices"
	"sort"
	"sync"
)

type client struct {
	user  user
	voice voiceState
	send  chan []byte
}

// voiceState é o que o cliente conta de si na call; Channel 0 é fora de call.
type voiceState struct {
	Channel  int64 `json:"channel"`
	Muted    bool  `json:"muted"`
	Deafened bool  `json:"deafened"`
	Sharing  bool  `json:"sharing"`
}

type hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

type presence struct {
	Name     string `json:"name"`
	Voice    int64  `json:"voice"`
	Muted    bool   `json:"muted"`
	Deafened bool   `json:"deafened"`
	Sharing  bool   `json:"sharing"`
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

func (h *hub) setVoice(c *client, state voiceState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	if state.Channel == 0 {
		state = voiceState{}
	}
	c.voice = state
	h.broadcastPresenceLocked()
}

func (h *hub) voiceOf(c *client) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return c.voice.Channel
}

// sendVoice entrega para quem está no canal de voz.
func (h *hub) sendVoice(channel int64, event any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var users []int64
	for c := range h.clients {
		if c.voice.Channel == channel {
			users = append(users, c.user.ID)
		}
	}
	if users != nil {
		h.sendLocked(users, event)
	}
}

func (h *hub) broadcast(event any) {
	h.sendTo(nil, event)
}

// sendTo entrega só para os usuários listados; nil entrega para todos.
func (h *hub) sendTo(users []int64, event any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sendLocked(users, event)
}

func (h *hub) broadcastLocked(event any) {
	h.sendLocked(nil, event)
}

func (h *hub) sendLocked(users []int64, event any) {
	b, _ := json.Marshal(event)
	dropped := false
	for c := range h.clients {
		if users != nil && !slices.Contains(users, c.user.ID) {
			continue
		}
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
	// Com o app aberto em dois lugares, vale a conexão que está em call.
	byName := map[string]voiceState{}
	for c := range h.clients {
		if c.voice.Channel != 0 || byName[c.user.Name].Channel == 0 {
			byName[c.user.Name] = c.voice
		}
	}
	users := make([]presence, 0, len(byName))
	for name, v := range byName {
		users = append(users, presence{name, v.Channel, v.Muted, v.Deafened, v.Sharing})
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	h.broadcastLocked(map[string]any{"type": "presence", "users": users})
}
