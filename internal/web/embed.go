// Package web embeds the UI templates and static assets.
package web

import "embed"

//go:embed templates/*.html static/*
var FS embed.FS
