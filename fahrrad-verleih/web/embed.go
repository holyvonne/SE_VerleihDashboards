// Package web bettet Templates und CSS in die ausführbare Datei ein (NF-9).
package web

import "embed"

//go:embed templates static
var FS embed.FS
