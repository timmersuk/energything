package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html static/*
var files embed.FS

func templates() (*template.Template, error) {
	return template.New("pages").Funcs(template.FuncMap{"sources": func() []string { return []string{"octopus", "growatt"} }, "streams": func() []string { return []string{"import", "export", "device", "generation"} }, "outcomes": func() []string {
		return []string{"running", "success", "empty", "rejected", "failed", "cancelled", "interrupted"}
	}, "errorClasses": func() []string {
		return []string{"configuration", "selection", "authentication", "rate_limit", "network", "timeout", "pagination", "protocol", "validation", "timestamp_quality", "storage"}
	}}).ParseFS(files, "templates/*.html")
}
