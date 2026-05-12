package codegen

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var tmpl *template.Template

func init() {
	funcMap := template.FuncMap{
		"renderTags": renderTags,
		"title":      strings.Title,
		"lower":      strings.ToLower,
	}
	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.tmpl"))
}

func renderTags(tags []Tag) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	for _, t := range tags {
		parts = append(parts, fmt.Sprintf(`%s:"%s"`, t.Key, t.Value))
	}
	return "`" + strings.Join(parts, " ") + "`"
}

type typesTemplateData struct {
	PackageName       string
	GenerateDirective string
	Imports           []string
	Types             []TypeDef
}

type serverTemplateData struct {
	PackageName       string
	GenerateDirective string
	Imports           []string
	Operations        []OperationDef
	ServerName        string
}

type specTemplateData struct {
	PackageName       string
	GenerateDirective string
	SpecBase64        string
}

type combinedTemplateData struct {
	PackageName       string
	GenerateDirective string
	Imports           []string
	Types             []TypeDef
	Operations        []OperationDef
	GenerateServer    bool
	ServerName        string
}

func executeTypesTemplate(data *typesTemplateData) (string, error) {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "types.go.tmpl", data); err != nil {
		return "", fmt.Errorf("execute types template: %w", err)
	}
	return buf.String(), nil
}

func executeServerTemplate(data *serverTemplateData) (string, error) {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "server.go.tmpl", data); err != nil {
		return "", fmt.Errorf("execute server template: %w", err)
	}
	return buf.String(), nil
}

func executeSpecTemplate(data *specTemplateData) (string, error) {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "spec.go.tmpl", data); err != nil {
		return "", fmt.Errorf("execute spec template: %w", err)
	}
	return buf.String(), nil
}

func executeCombinedTemplate(data *combinedTemplateData) (string, error) {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "file.go.tmpl", data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
