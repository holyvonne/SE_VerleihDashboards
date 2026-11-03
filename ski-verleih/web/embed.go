// Package web bettet Templates und statische Dateien in die Binary ein (NF-9).
package web

import "embed"

//go:embed templates static
var FS embed.FS
