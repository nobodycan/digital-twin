package presentation

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

var ErrEmptySpeechText = errors.New("speech text is empty")

type TimelineOptions struct {
	StartAt time.Time
}

type SpeechTimeline struct {
	Subtitles []SubtitleSegment `json:"subtitles"`
	Visemes   []VisemeMarker    `json:"visemes,omitempty"`
}

type SubtitleSegment struct {
	Text  string    `json:"text"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type VisemeMarker struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
}

func BuildSpeechTimeline(text string, options TimelineOptions) (SpeechTimeline, error) {
	segments := splitSubtitleText(text)
	if len(segments) == 0 {
		return SpeechTimeline{}, ErrEmptySpeechText
	}

	cursor := options.StartAt
	subtitles := make([]SubtitleSegment, 0, len(segments))
	visemes := make([]VisemeMarker, 0, len(segments))
	for _, segment := range segments {
		duration := estimateSpeechDuration(segment)
		end := cursor.Add(duration)
		subtitles = append(subtitles, SubtitleSegment{
			Text:  segment,
			Start: cursor,
			End:   end,
		})
		visemes = append(visemes, VisemeMarker{Name: simplifiedViseme(segment), At: cursor})
		cursor = end
	}

	return SpeechTimeline{Subtitles: subtitles, Visemes: visemes}, nil
}

func splitSubtitleText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var segments []string
	start := 0
	for index, value := range text {
		if value == '.' || value == '!' || value == '?' || value == '。' || value == '！' || value == '？' {
			segment := strings.TrimSpace(text[start : index+len(string(value))])
			if segment != "" {
				segments = append(segments, segment)
			}
			start = index + len(string(value))
		}
	}
	if start < len(text) {
		segment := strings.TrimSpace(text[start:])
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func estimateSpeechDuration(text string) time.Duration {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 500 * time.Millisecond
	}
	duration := time.Duration(len(words)) * 350 * time.Millisecond
	if duration < 700*time.Millisecond {
		return 700 * time.Millisecond
	}
	return duration
}

func simplifiedViseme(text string) string {
	for _, value := range strings.ToLower(text) {
		if !unicode.IsLetter(value) {
			continue
		}
		switch value {
		case 'a', 'e', 'i', 'o', 'u':
			return "open"
		default:
			return "closed"
		}
	}
	return "neutral"
}
