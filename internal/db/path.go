package db

import "github.com/adrg/xdg"

// DefaultPath is "wisp-go", not "wisp" — the old rust-tauri branch used an
// incompatible schema at that path.
func DefaultPath() (string, error) {
	return xdg.DataFile("wisp-go/wisp.db")
}
