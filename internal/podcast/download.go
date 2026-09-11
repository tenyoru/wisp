package podcast

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"wisp/internal/httpx"
	"wisp/internal/paths"
)

const progressInterval = 250 * time.Millisecond

var unsafeNameChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]`)

func sanitizeName(name string) string {
	name = unsafeNameChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, " .")
	if name == "" {
		return "untitled"
	}
	if len(name) > 150 {
		name = name[:150]
	}
	return name
}

func podcastDir(podcastName string) string {
	return sanitizeName(podcastName)
}

func episodePath(podcastName, episodeName, audioURL string) string {
	ext := path.Ext(strings.SplitN(audioURL, "?", 2)[0])
	switch ext {
	case "":
		ext = ".mp3"
	}
	return podcastDir(podcastName) + "/" + sanitizeName(episodeName) + ext
}

func DownloadEpisode(ctx context.Context, podcastName, episodeName, audioURL string, onProgress func(downloaded, total int64)) (string, error) {
	relPath := episodePath(podcastName, episodeName, audioURL)
	dest := filepath.Join(paths.EpisodesDir(), filepath.FromSlash(relPath))
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return relPath, nil
	}
	cache := filepath.Join(paths.Cache, "episodes", filepath.FromSlash(relPath))
	if st, err := os.Stat(cache); err == nil && st.Size() > 0 {
		if onProgress != nil {
			onProgress(st.Size(), st.Size())
		}
		if err := promote(cache, dest); err != nil {
			return "", err
		}
		return relPath, nil
	}
	if err := downloadTo(ctx, dest, audioURL, onProgress); err != nil {
		return "", err
	}
	return relPath, nil
}

func EnsureCached(ctx context.Context, podcastName, episodeName, audioURL string) (string, error) {
	relPath := episodePath(podcastName, episodeName, audioURL)
	dest := filepath.Join(paths.Cache, "episodes", filepath.FromSlash(relPath))
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return dest, nil
	}
	if err := downloadTo(ctx, dest, audioURL, nil); err != nil {
		return "", err
	}
	return dest, nil
}

func promote(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(dest)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	os.Remove(src)
	return nil
}

func downloadTo(ctx context.Context, dest, audioURL string, onProgress func(downloaded, total int64)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", httpx.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: unexpected status %s", audioURL, resp.Status)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	pw := &progressWriter{total: resp.ContentLength, onProgress: onProgress}
	_, copyErr := io.Copy(f, io.TeeReader(resp.Body, pw))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func DeleteEpisode(relPath string) error {
	if err := os.Remove(filepath.Join(paths.EpisodesDir(), filepath.FromSlash(relPath))); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DeletePodcastDir(podcastName string) error {
	return os.RemoveAll(filepath.Join(paths.EpisodesDir(), podcastDir(podcastName)))
}

type progressWriter struct {
	total      int64
	downloaded int64
	lastEmit   time.Time
	onProgress func(downloaded, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.downloaded += int64(len(p))
	if w.onProgress != nil && time.Since(w.lastEmit) >= progressInterval {
		w.lastEmit = time.Now()
		w.onProgress(w.downloaded, w.total)
	}
	return len(p), nil
}
