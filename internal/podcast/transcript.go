package podcast

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"wisp/internal/httpx"
)

var (
	timingLine  = regexp.MustCompile(`-->`)
	numericLine = regexp.MustCompile(`^\d+$`)
	voiceTag    = regexp.MustCompile(`(?i)^<v(?:\.[\w-]+)*\s+([^>]+)>\s*`)
)

func FetchTranscript(ctx context.Context, url, mimeType string) (string, error) {
	body, _, err := httpx.FetchCapped(ctx, url, httpx.MaxBytes)
	if err != nil {
		return "", err
	}
	text := string(body)

	switch {
	case strings.Contains(mimeType, "vtt"), strings.Contains(mimeType, "srt"), strings.Contains(mimeType, "subrip"):
		return plainTextFromSubtitles(text), nil
	case strings.Contains(mimeType, "plain"):
		return text, nil
	default:
		return "", fmt.Errorf("unsupported transcript type %q", mimeType)
	}
}

// minParagraphLen is a floor, not a target — a paragraph only breaks once
// it's both past this length and at a sentence boundary, so a cue never
// splits mid-sentence.
const minParagraphLen = 280

type cue struct {
	speaker string // empty when the source has no WebVTT <v> voice tags
	text    string
}

func parseCues(body string) []cue {
	var cues []cue
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "WEBVTT") || timingLine.MatchString(line) || numericLine.MatchString(line) {
			continue
		}
		speaker := ""
		if m := voiceTag.FindStringSubmatch(line); m != nil {
			speaker = strings.TrimSpace(m[1])
			line = strings.TrimSpace(line[len(m[0]):])
		}
		line = strings.TrimSuffix(line, "</v>")
		cues = append(cues, cue{speaker: speaker, text: line})
	}
	return cues
}

func plainTextFromSubtitles(body string) string {
	cues := parseCues(body)
	for _, c := range cues {
		if c.speaker != "" {
			return formatDialogue(cues)
		}
	}
	return formatProse(cues)
}

// formatDialogue starts a new paragraph on every speaker change, prefixed
// with their name — a straight transcription otherwise reads as one voice.
func formatDialogue(cues []cue) string {
	var paragraphs []string
	var cur strings.Builder
	lastSpeaker := ""
	for _, c := range cues {
		if c.speaker != lastSpeaker && cur.Len() > 0 {
			paragraphs = append(paragraphs, cur.String())
			cur.Reset()
		}
		if cur.Len() == 0 && c.speaker != "" {
			cur.WriteString("**" + c.speaker + ":** ")
		} else if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(c.text)
		lastSpeaker = c.speaker
	}
	if cur.Len() > 0 {
		paragraphs = append(paragraphs, cur.String())
	}
	return strings.Join(paragraphs, "\n\n")
}

func formatProse(cues []cue) string {
	var paragraphs []string
	var cur strings.Builder
	for _, c := range cues {
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(c.text)
		if cur.Len() >= minParagraphLen && endsSentence(c.text) {
			paragraphs = append(paragraphs, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		paragraphs = append(paragraphs, cur.String())
	}
	return strings.Join(paragraphs, "\n\n")
}

func endsSentence(text string) bool {
	text = strings.TrimRight(text, `"')]`)
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?")
}
