package deploy

import (
	"sync"
	"time"
)

// Event is the small, sanitized stream sent to the UI while a deployment runs.
// It never contains credentials or arbitrary process environment values.
type Event struct {
	Type         string `json:"type"`
	DeploymentID string `json:"deployment_id"`
	Channel      string `json:"channel,omitempty"`
	Status       string `json:"status,omitempty"`
	Message      string `json:"message,omitempty"`
	At           string `json:"at"`
}

type EventHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{subscribers: make(map[string]map[chan Event]struct{})}
}

func (h *EventHub) Subscribe(deploymentID string) (<-chan Event, func()) {
	channel := make(chan Event, 32)
	h.mu.Lock()
	if h.subscribers[deploymentID] == nil {
		h.subscribers[deploymentID] = make(map[chan Event]struct{})
	}
	h.subscribers[deploymentID][channel] = struct{}{}
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		if subscribers := h.subscribers[deploymentID]; subscribers != nil {
			if _, ok := subscribers[channel]; ok {
				delete(subscribers, channel)
				close(channel)
			}
			if len(subscribers) == 0 {
				delete(h.subscribers, deploymentID)
			}
		}
		h.mu.Unlock()
	}
}

func (h *EventHub) Publish(event Event) {
	if h == nil {
		return
	}
	if event.At == "" {
		event.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for subscriber := range h.subscribers[event.DeploymentID] {
		select {
		case subscriber <- event:
		default:
			// A slow browser must not block the deployment worker.
		}
	}
}
