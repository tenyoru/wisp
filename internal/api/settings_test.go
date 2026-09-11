package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	got, err := ReadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSettings() {
		t.Fatalf("missing file: %+v", got)
	}

	in := Settings{Theme: "light", PlaybackSpeed: 1.5, WhisperModel: "small", CloudTranscription: true}
	if err := WriteSettings(path, in); err != nil {
		t.Fatal(err)
	}
	got, err = ReadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("roundtrip %+v, want %+v", got, in)
	}

	if err := os.WriteFile(path, []byte(`{"theme":"neon","playbackSpeed":99,"whisperModel":"huge","extra":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ReadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSettings() {
		t.Fatalf("clamp %+v, want defaults", got)
	}
}
