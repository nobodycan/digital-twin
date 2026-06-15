package runtime

import (
	"context"
	"sync"
)

// Event is a runtime event emitted between subsystems.
type Event struct {
	Topic   string
	Payload any
}

// EventBus publishes events to topic subscribers.
type EventBus interface {
	Publish(context.Context, Event) error
	Subscribe(string) (<-chan Event, func())
}

// LocalEventBus is an in-process event bus.
type LocalEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]struct{}
}

// NewEventBus creates an in-process event bus.
func NewEventBus() *LocalEventBus {
	return &LocalEventBus{subscribers: make(map[string]map[chan Event]struct{})}
}

// Publish sends an event to current subscribers of its topic.
func (b *LocalEventBus) Publish(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.RLock()
	subscribers := make([]chan Event, 0, len(b.subscribers[event.Topic]))
	for subscriber := range b.subscribers[event.Topic] {
		subscribers = append(subscribers, subscriber)
	}
	b.mu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Subscribe registers a subscriber channel for topic.
func (b *LocalEventBus) Subscribe(topic string) (<-chan Event, func()) {
	channel := make(chan Event, 1)
	b.mu.Lock()
	if b.subscribers[topic] == nil {
		b.subscribers[topic] = make(map[chan Event]struct{})
	}
	b.subscribers[topic][channel] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subscribers[topic], channel)
		close(channel)
		b.mu.Unlock()
	}
	return channel, cancel
}

var _ EventBus = (*LocalEventBus)(nil)
