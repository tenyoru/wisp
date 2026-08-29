package podcast

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	timingLine  = regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[.,](\d{3})\s*-->`)
	numericLine = regexp.MustCompile(`^\d+$`)
	voiceTag    = regexp.MustCompile(`(?i)^<v(?:\.[\w-]+)*\s+([^>]+)>\s*`)
)

// minParagraphLen is a floor, not a target — a paragraph only breaks once
// it's both past this length and at a sentence boundary, so a cue never
// splits mid-sentence.
const minParagraphLen = 280

type cue struct {
	start   float64 // seconds, from the cue's own timing line
	speaker string  // empty when the source has no WebVTT <v> voice tags
	text    string
}

func parseCues(body string) []cue {
	var cues []cue
	currentStart := 0.0
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "WEBVTT") || numericLine.MatchString(line) {
			continue
		}
		if m := timingLine.FindStringSubmatch(line); m != nil {
			currentStart = parseTimestamp(m)
			continue
		}

		speaker := ""
		if m := voiceTag.FindStringSubmatch(line); m != nil {
			speaker = strings.TrimSpace(m[1])
			line = strings.TrimSpace(line[len(m[0]):])
		}
		line = strings.TrimSuffix(line, "</v>")
		cues = append(cues, cue{start: currentStart, speaker: speaker, text: line})
	}
	return cues
}

func parseTimestamp(m []string) float64 {
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	ms, _ := strconv.Atoi(m[4])
	return float64(h*3600+min*60+sec) + float64(ms)/1000
}

// formatTimestamp renders seconds as a player-style clock, and timeLink
// wraps that as a markdown link the frontend intercepts to seek playback
// (see loadShowNotes's click handler on #t= links) instead of navigating.
func formatTimestamp(seconds float64) string {
	total := int(seconds)
	h, min, sec := total/3600, (total/60)%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, min, sec)
	}
	return fmt.Sprintf("%d:%02d", min, sec)
}

func timeLink(seconds float64) string {
	return fmt.Sprintf("[%s](#t=%d)", formatTimestamp(seconds), int(seconds))
}

// wraps one cue in its own #t= link, finer-grained than the paragraph badge
func cueLink(c cue) string {
	return "[" + escapeLinkText(c.text) + "](#t=" + strconv.Itoa(int(c.start)) + ")"
}

func escapeLinkText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `]`, `\]`)
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
	var start float64
	lastSpeaker := ""
	for _, c := range cues {
		if c.speaker != lastSpeaker && cur.Len() > 0 {
			paragraphs = append(paragraphs, timeLink(start)+" "+cur.String())
			cur.Reset()
		}
		if cur.Len() == 0 {
			start = c.start
			if c.speaker != "" {
				cur.WriteString("**" + c.speaker + ":** ")
			}
		} else {
			cur.WriteByte(' ')
		}
		cur.WriteString(cueLink(c))
		lastSpeaker = c.speaker
	}
	if cur.Len() > 0 {
		paragraphs = append(paragraphs, timeLink(start)+" "+cur.String())
	}
	return strings.Join(paragraphs, "\n\n")
}

func formatProse(cues []cue) string {
	var paragraphs []string
	var cur strings.Builder
	var start float64
	visibleLen := 0
	for _, c := range cues {
		if cur.Len() == 0 {
			start = c.start
		} else {
			cur.WriteByte(' ')
			visibleLen++
		}
		cur.WriteString(cueLink(c))
		visibleLen += len(c.text)
		if visibleLen >= minParagraphLen && endsSentence(c.text) {
			paragraphs = append(paragraphs, timeLink(start)+" "+cur.String())
			cur.Reset()
			visibleLen = 0
		}
	}
	if cur.Len() > 0 {
		paragraphs = append(paragraphs, timeLink(start)+" "+cur.String())
	}
	return strings.Join(paragraphs, "\n\n")
}

func endsSentence(text string) bool {
	text = strings.TrimRight(text, `"')]`)
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?")
}
