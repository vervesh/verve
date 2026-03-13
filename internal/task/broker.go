package task

import (
	"context"
	"encoding/json"
	"sync"
)

// EventType identifies the kind of SSE event.
const (
	EventTaskCreated  = "task_created"
	EventTaskUpdated  = "task_updated"
	EventTaskDeleted  = "task_deleted"
	EventLogsAppended = "logs_appended"
	EventRepoUpdated  = "repo_updated"
)

// Event represents a task or repo mutation broadcast to SSE subscribers.
type Event struct {
	Type    string   `json:"type"`
	RepoID  string   `json:"repo_id,omitempty"`
	Task    *Task    `json:"task,omitempty"`
	TaskID  TaskID   `json:"task_id,omitempty"`
	Logs    []string `json:"logs,omitempty"`
	Attempt int      `json:"attempt,omitempty"`
	Repo    any      `json:"repo,omitempty"`
}

// Notifier sends event payloads to an external notification system for
// cross-instance fan-out. The listen side calls Broker.Receive to complete
// the fan-out.
type Notifier interface {
	Notify(ctx context.Context, payload []byte) error
}

// Broker fans out task events to SSE subscribers. When a Notifier is
// configured, events are routed through the external notification system
// so that multiple server instances can share events. When nil, events
// are fanned out directly in-process.
type Broker struct {
	mu       sync.RWMutex
	subs     map[chan Event]struct{}
	notifier Notifier
}

// NewBroker creates a new Broker. If notifier is non-nil, Publish sends
// events through the external notification system; otherwise events are
// fanned out locally.
func NewBroker(notifier Notifier) *Broker {
	return &Broker{
		subs:     make(map[chan Event]struct{}),
		notifier: notifier,
	}
}

// Subscribe returns a buffered channel that receives task events.
func (b *Broker) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (b *Broker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends an event. If a notifier is configured, the event is
// serialized and sent through the external notification system (the
// LISTEN side calls Receive to fan out locally). Otherwise, fans out
// directly to local subscribers.
func (b *Broker) Publish(ctx context.Context, event Event) {
	if b.notifier != nil {
		payload, err := json.Marshal(event)
		if err != nil {
			return
		}
		_ = b.notifier.Notify(ctx, payload)
		return
	}
	b.fanOut(event)
}

// Receive is called by an external listener (e.g., PG LISTEN loop) to
// inject an event for local fan-out to SSE subscribers.
func (b *Broker) Receive(event Event) {
	b.fanOut(event)
}

func (b *Broker) fanOut(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}
