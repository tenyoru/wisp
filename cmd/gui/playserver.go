package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wisp/internal/db"
	"wisp/internal/paths"
	"wisp/internal/podcast"
)

const episodeListen = "127.0.0.1:9246"

func playServerOurs() bool {
	c := &http.Client{Timeout: 150 * time.Millisecond}
	req, err := http.NewRequest(http.MethodOptions, "http://"+episodeListen+"/play/", nil)
	if err != nil {
		return false
	}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.Header.Get("X-Wisp-Play") != ""
}

func serveEpisodes(h http.Handler) {
	if playServerOurs() {
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/play/", h)
	if err := http.ListenAndServe(episodeListen, mux); err != nil {
		log.Printf("wisp: episode server: %v", err)
	}
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Wisp-Play", "1")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

type playServer struct {
	store db.Store
}

func (s *playServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/play/"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	item, err := s.store.GetItem(ctx, id)
	if err != nil || item == nil || item.AudioURL == "" {
		http.NotFound(w, r)
		return
	}
	file := ""
	if item.DownloadFilename != "" {
		p := filepath.Join(paths.EpisodesDir(), filepath.FromSlash(item.DownloadFilename))
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			file = p
		}
	}
	if file == "" {
		feed, err := s.store.GetFeed(ctx, item.FeedID)
		if err != nil || feed == nil {
			http.Error(w, "no feed", http.StatusInternalServerError)
			return
		}
		file, err = podcast.EnsureCached(ctx, feed.Title, item.Title, item.AudioURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	http.ServeFile(w, r, file)
}
