// Package paths resolves wisp's on-disk locations, under the OS-appropriate
// app-data/cache dirs (via github.com/adrg/xdg).
package paths

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

var (
	DB       string
	Episodes string
)

// Call once at startup, before anything reads DB/Episodes.
func Resolve() error {
	// "wisp-go", not "wisp" — the old rust-tauri branch used an
	// incompatible schema at that path.
	db, err := xdg.DataFile("wisp-go/wisp.db")
	if err != nil {
		return err
	}
	DB = db

	placeholder, err := xdg.DataFile(filepath.Join("podcasts", ".keep"))
	if err != nil {
		return err
	}
	Episodes = filepath.Dir(placeholder)
	return nil
}

func EpisodesDir() string {
	return Episodes
}
