//go:build !nohtmlui
// +build !nohtmlui

package server

import (
	"net/http"

	"github.com/blinkdisk/htmluibuild"
)

// AssetFile exposes HTML UI files.
func AssetFile() http.FileSystem {
	return htmluibuild.AssetFile()
}
