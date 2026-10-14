package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html static/*
var files embed.FS

func templates() (*template.Template, error) { return template.ParseFS(files, "templates/*.html") }
