package db

import "github.com/adrg/xdg"

func DefaultPath() (string, error) {
	return xdg.DataFile("wisp-go/wisp.db")
}
