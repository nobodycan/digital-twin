package persona

import "strings"

const (
	streamGuardPendingLimit   = 4 * 1024
	streamGuardCandidateLimit = 256 * 1024
)

type StreamGuard struct {
	guard         Guard
	confidence    float64
	fullBuffer    bool
	pending       string
	complete      string
	visibleOutput bool
}

type StreamGuardStep struct {
	Segments []string
	Decision GuardDecision
	Buffered bool
}

func NewStreamGuard(guard Guard, confidence float64) *StreamGuard {
	return &StreamGuard{
		guard:      guard,
		confidence: confidence,
		fullBuffer: confidence < 0.5,
	}
}

func (g *StreamGuard) Add(chunk string) StreamGuardStep {
	if chunk == "" {
		return StreamGuardStep{Decision: GuardDecision{Allowed: true, Reason: "ok"}, Buffered: g.fullBuffer}
	}

	g.pending += chunk
	g.complete += chunk

	if len(g.complete) > streamGuardCandidateLimit {
		return StreamGuardStep{
			Decision: GuardDecision{
				Allowed:      false,
				Reason:       "candidate_too_large",
				SafeFallback: "I couldn't finish that safely. Please try a shorter request.",
			},
			Buffered: g.fullBuffer,
		}
	}

	if g.fullBuffer {
		return StreamGuardStep{Decision: GuardDecision{Allowed: true, Reason: "ok"}, Buffered: true}
	}

	if decision := g.guard.Check(g.pending, 0.9); !decision.Allowed {
		return StreamGuardStep{Decision: decision}
	}

	if len(g.pending) > streamGuardPendingLimit {
		g.fullBuffer = true
		return StreamGuardStep{Decision: GuardDecision{Allowed: true, Reason: "ok"}, Buffered: true}
	}

	boundary := lastSentenceBoundary(g.pending)
	if boundary == 0 {
		return StreamGuardStep{Decision: GuardDecision{Allowed: true, Reason: "ok"}}
	}

	segment := g.pending[:boundary]
	g.pending = g.pending[boundary:]
	g.visibleOutput = g.visibleOutput || strings.TrimSpace(segment) != ""
	return StreamGuardStep{
		Segments: []string{segment},
		Decision: GuardDecision{Allowed: true, Reason: "ok"},
	}
}

func (g *StreamGuard) Finalize() StreamGuardStep {
	decision := g.guard.Check(g.complete, g.confidence)
	if !decision.Allowed {
		return StreamGuardStep{Decision: decision, Buffered: g.fullBuffer}
	}

	if g.pending == "" {
		return StreamGuardStep{Decision: decision, Buffered: g.fullBuffer}
	}

	segments := []string{g.pending}
	g.visibleOutput = g.visibleOutput || strings.TrimSpace(g.pending) != ""
	g.pending = ""
	return StreamGuardStep{Segments: segments, Decision: decision, Buffered: g.fullBuffer}
}

func (g *StreamGuard) HasVisibleOutput() bool {
	return g.visibleOutput
}

func lastSentenceBoundary(value string) int {
	for i := len(value) - 1; i >= 0; i-- {
		switch value[i] {
		case '.', '!', '?', '\n':
			return i + 1
		}
	}
	return 0
}
