package presentation

import (
	"testing"
	"time"
)

func TestBuildSpeechTimelineSplitsTextIntoTimedSubtitles(t *testing.T) {
	start := time.Date(2026, 6, 17, 16, 0, 0, 0, time.UTC)

	timeline, err := BuildSpeechTimeline("Hello there. I can help.", TimelineOptions{
		StartAt: start,
	})
	if err != nil {
		t.Fatalf("BuildSpeechTimeline returned error: %v", err)
	}

	if len(timeline.Subtitles) != 2 {
		t.Fatalf("subtitle count = %d, want 2", len(timeline.Subtitles))
	}
	if timeline.Subtitles[0].Text != "Hello there." {
		t.Fatalf("first subtitle = %q", timeline.Subtitles[0].Text)
	}
	if !timeline.Subtitles[0].Start.Equal(start) {
		t.Fatalf("first subtitle start = %s, want %s", timeline.Subtitles[0].Start, start)
	}
	if !timeline.Subtitles[0].End.After(timeline.Subtitles[0].Start) {
		t.Fatalf("first subtitle should have positive duration")
	}
	if !timeline.Subtitles[1].Start.Equal(timeline.Subtitles[0].End) {
		t.Fatalf("second subtitle should start where first ends")
	}
	if len(timeline.Visemes) == 0 {
		t.Fatalf("expected simplified viseme markers")
	}
}

func TestBuildSpeechTimelineRejectsEmptyText(t *testing.T) {
	_, err := BuildSpeechTimeline("   ", TimelineOptions{StartAt: time.Now()})
	if err == nil {
		t.Fatalf("expected empty text to be rejected")
	}
}
