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

func plainTextFromSubtitles(body string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "WEBVTT") || timingLine.MatchString(line) || numericLine.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
