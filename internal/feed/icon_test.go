package feed

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestFetchIcon_DiscoversLinkTag(t *testing.T) {
	icon := tinyPNG(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><link rel="shortcut icon" href="/assets/icon.png"></head><body></body></html>`))
	})
	mux.HandleFunc("/assets/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon)
	})
	// A favicon.ico also exists, to prove the <link> tag wins when present.
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		t.Error("favicon.ico fallback was fetched even though a <link rel=icon> was found")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	data, mimeType, err := FetchIcon(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchIcon: %v", err)
	}
	if !bytes.Equal(data, icon) {
		t.Errorf("got %d bytes, want the %d-byte fixture icon", len(data), len(icon))
	}
	if mimeType != "image/png" {
		t.Errorf("mimeType = %q, want %q", mimeType, "image/png")
	}
}

func TestFetchIcon_FallsBackToFaviconIco(t *testing.T) {
	icon := tinyPNG(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>No icon link here</title></head><body></body></html>`))
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(icon)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	data, mimeType, err := FetchIcon(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchIcon: %v", err)
	}
	if !bytes.Equal(data, icon) {
		t.Errorf("got %d bytes, want the %d-byte fixture icon", len(data), len(icon))
	}
	if mimeType != "image/x-icon" {
		t.Errorf("mimeType = %q, want %q", mimeType, "image/x-icon")
	}
}

func TestFetchIcon_NoIconAnywhere(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if _, _, err := FetchIcon(context.Background(), srv.URL); err == nil {
		t.Fatal("FetchIcon: expected an error when neither a <link> icon nor /favicon.ico exists")
	}
}
