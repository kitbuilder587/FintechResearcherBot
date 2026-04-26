package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*
var templatesFS embed.FS

func Text(name string) string {
	content, err := templatesFS.ReadFile("templates/" + name)
	if err != nil {
		panic(fmt.Sprintf("read prompt %s: %v", name, err))
	}
	return strings.TrimSpace(string(content))
}

func Render(name string, data any) (string, error) {
	tmpl, err := template.New(name).Parse(Text(name))
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", name, err)
	}

	return strings.TrimSpace(buf.String()), nil
}
