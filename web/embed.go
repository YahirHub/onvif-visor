package web

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var assets embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(assets, "static")
}
