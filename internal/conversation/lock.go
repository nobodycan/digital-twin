package conversation

import "sync"

type ActiveConversationGate struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func NewActiveConversationGate() *ActiveConversationGate {
	return &ActiveConversationGate{
		active: make(map[string]struct{}),
	}
}

func (g *ActiveConversationGate) TryAcquire(key string) (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.active[key]; ok {
		return nil, false
	}
	g.active[key] = struct{}{}
	return func() {
		g.mu.Lock()
		delete(g.active, key)
		g.mu.Unlock()
	}, true
}
