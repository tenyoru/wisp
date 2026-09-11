package paths

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

var (
	DB       string
	Episodes string
	Settings string
)

func Resolve() error {
	db, err := xdg.DataFile("wisp/wisp.db")
	if err != nil {
		return err
	}
	DB = db

	placeholder, err := xdg.DataFile(filepath.Join("podcasts", ".keep"))
	if err != nil {
		return err
	}
	Episodes = filepath.Dir(placeholder)

	cfg, err := xdg.DataFile("wisp/settings.json")
	if err != nil {
		return err
	}
	Settings = cfg
	return nil
}

func EpisodesDir() string {
	return Episodes
}
