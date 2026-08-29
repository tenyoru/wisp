package podcast

import (
	"context"
	"fmt"
	"strings"

	"wisp/internal/httpx"
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
