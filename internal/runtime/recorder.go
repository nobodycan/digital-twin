package runtime

import "sync"

type EventRecorder struct {
	mu     sync.Mutex
	events []RuntimeEvent
}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (r *EventRecorder) Record(event RuntimeEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *EventRecorder) Events() []RuntimeEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RuntimeEvent(nil), r.events...)
}
