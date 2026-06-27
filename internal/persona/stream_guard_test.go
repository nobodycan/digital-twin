package persona

import "testing"

func TestStreamGuardEmitsAcceptedSentenceSegments(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.9)

	first := guard.Add("Hello there.")
	if !first.Decision.Allowed {
		t.Fatalf("first decision = %+v, want allowed", first.Decision)
	}
	if len(first.Segments) != 1 || first.Segments[0] != "Hello there." {
		t.Fatalf("first segments = %#v, want one complete sentence", first.Segments)
	}

	final := guard.Finalize()
	if !final.Decision.Allowed {
		t.Fatalf("final decision = %+v, want allowed", final.Decision)
	}
	if len(final.Segments) != 0 {
		t.Fatalf("final segments = %#v, want no trailing output", final.Segments)
	}
}

func TestStreamGuardRejectsForbiddenClaimInsideOneChunk(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.9)

	step := guard.Add("I can guarantee investment returns.")
	if step.Decision.Allowed {
		t.Fatalf("decision = %+v, want forbidden claim rejection", step.Decision)
	}
	if step.Decision.Reason != "forbidden_claim" {
		t.Fatalf("reason = %q, want forbidden_claim", step.Decision.Reason)
	}
	if len(step.Segments) != 0 {
		t.Fatalf("segments = %#v, want nothing emitted", step.Segments)
	}
}

func TestStreamGuardRejectsForbiddenClaimSplitAcrossChunks(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.9)

	first := guard.Add("I can guarantee invest")
	if !first.Decision.Allowed {
		t.Fatalf("first decision = %+v, want allowed pending buffer", first.Decision)
	}
	if len(first.Segments) != 0 {
		t.Fatalf("first segments = %#v, want nothing emitted yet", first.Segments)
	}

	second := guard.Add("ment returns soon.")
	if second.Decision.Allowed {
		t.Fatalf("second decision = %+v, want forbidden claim rejection", second.Decision)
	}
	if len(second.Segments) != 0 {
		t.Fatalf("second segments = %#v, want nothing emitted", second.Segments)
	}
}

func TestStreamGuardRetainsSuffixUntilSentenceBoundary(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.9)

	first := guard.Add("Hello wor")
	if len(first.Segments) != 0 {
		t.Fatalf("first segments = %#v, want none", first.Segments)
	}

	second := guard.Add("ld. Next")
	if len(second.Segments) != 1 || second.Segments[0] != "Hello world." {
		t.Fatalf("second segments = %#v, want released completed sentence", second.Segments)
	}

	final := guard.Finalize()
	if len(final.Segments) != 1 || final.Segments[0] != " Next" {
		t.Fatalf("final segments = %#v, want retained suffix", final.Segments)
	}
}

func TestStreamGuardUsesFullBufferForLowConfidence(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.2)

	step := guard.Add("This might work.")
	if len(step.Segments) != 0 {
		t.Fatalf("segments = %#v, want full buffering", step.Segments)
	}
	if !step.Buffered {
		t.Fatal("Buffered = false, want true for low confidence path")
	}

	final := guard.Finalize()
	if !final.Decision.Allowed {
		t.Fatalf("final decision = %+v, want allowed", final.Decision)
	}
	if len(final.Segments) != 1 || final.Segments[0] != "This might work." {
		t.Fatalf("final segments = %#v, want buffered release", final.Segments)
	}
}

func TestStreamGuardFinalGuardRejectsBeforeVisibleOutput(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.2)

	step := guard.Add("This is the exact answer")
	if len(step.Segments) != 0 {
		t.Fatalf("segments = %#v, want full buffering", step.Segments)
	}

	final := guard.Finalize()
	if final.Decision.Allowed {
		t.Fatalf("final decision = %+v, want missing_uncertainty rejection", final.Decision)
	}
	if guard.HasVisibleOutput() {
		t.Fatal("HasVisibleOutput = true, want false")
	}
}

func TestStreamGuardRejectsAfterVisibleOutputWhenLaterChunksTurnUnsafe(t *testing.T) {
	guard := NewStreamGuard(Guard{Persona: validPersona()}, 0.9)

	first := guard.Add("Planning has tradeoffs.")
	if len(first.Segments) != 1 {
		t.Fatalf("first segments = %#v, want visible output", first.Segments)
	}

	second := guard.Add(" I can guarantee investment")
	if !second.Decision.Allowed {
		t.Fatalf("second decision = %+v, want pending until finalize", second.Decision)
	}

	third := guard.Add(" returns")
	if third.Decision.Allowed {
		t.Fatalf("third decision = %+v, want forbidden claim rejection", third.Decision)
	}
	if !guard.HasVisibleOutput() {
		t.Fatal("HasVisibleOutput = false, want true")
	}
}
