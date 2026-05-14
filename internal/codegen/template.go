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
		"renderTags":         renderTags,
		"title":              strings.Title,
		"lower":              strings.ToLower,
		"methodCall":         methodCall,
		"pathParams":         filterPathParams,
		"queryParams":        filterQueryParams,
		"headerParams":       filterHeaderParams,
		"bodyFields":         filterBodyFields,
		"tagValue":           tagValue,
		"isPointerType":      isPointerType,
		"fmtValue":           fmtValue,
		"fmtDerefValue":      fmtDerefValue,
		"clientRspType":      clientRspType,
		"clientRspSignature": clientRspSignature,
		"skipForClient":      skipForClient,
		"zeroReturn":         zeroReturn,
		"successReturn":      successReturn,
		"needsResult":        needsResult,
		"isFileRsp":          isFileRsp,
		"isStringRsp":        isStringRsp,
		"hasSSEOps":          hasSSEOps,
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

func methodCall(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "Get"
	case "POST":
		return "Post"
	case "PUT":
		return "Put"
	case "PATCH":
		return "Patch"
	case "DELETE":
		return "Delete"
	case "HEAD":
		return "Head"
	case "OPTIONS":
		return "Options"
	default:
		return strings.Title(strings.ToLower(method))
	}
}

func filterPathParams(req *StructDef) []FieldDef {
	if req == nil {
		return nil
	}
	var result []FieldDef
	for _, f := range req.Fields {
		for _, t := range f.Tags {
			if t.Key == "uri" {
				result = append(result, f)
				break
			}
		}
	}
	return result
}

func filterQueryParams(req *StructDef) []FieldDef {
	if req == nil {
		return nil
	}
	var result []FieldDef
	for _, f := range req.Fields {
		for _, t := range f.Tags {
			if t.Key == "form" {
				result = append(result, f)
				break
			}
		}
	}
	return result
}

func filterHeaderParams(req *StructDef) []FieldDef {
	if req == nil {
		return nil
	}
	var result []FieldDef
	for _, f := range req.Fields {
		for _, t := range f.Tags {
			if t.Key == "header" {
				result = append(result, f)
				break
			}
		}
	}
	return result
}

func filterBodyFields(req *StructDef) []FieldDef {
	if req == nil {
		return nil
	}
	var result []FieldDef
	for _, f := range req.Fields {
		for _, t := range f.Tags {
			if t.Key == "json" {
				result = append(result, f)
				break
			}
		}
	}
	return result
}

func tagValue(f FieldDef, key string) string {
	for _, t := range f.Tags {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}

func isPointerType(f FieldDef) bool {
	return len(f.Type) > 0 && f.Type[0] == '*'
}

func fmtValue(f FieldDef) string {
	baseType := f.Type
	if len(baseType) > 0 && baseType[0] == '*' {
		baseType = baseType[1:]
	}
	switch baseType {
	case "string":
		return "req." + f.Name
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return fmt.Sprintf("fmt.Sprintf(\"%%d\", req.%s)", f.Name)
	case "bool":
		return fmt.Sprintf("fmt.Sprintf(\"%%t\", req.%s)", f.Name)
	case "float32", "float64":
		return fmt.Sprintf("fmt.Sprintf(\"%%g\", req.%s)", f.Name)
	default:
		return fmt.Sprintf("fmt.Sprintf(\"%%v\", req.%s)", f.Name)
	}
}

func fmtDerefValue(f FieldDef) string {
	baseType := f.Type
	if len(baseType) > 0 && baseType[0] == '*' {
		baseType = baseType[1:]
	}
	switch baseType {
	case "string":
		return "*req." + f.Name
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return fmt.Sprintf("fmt.Sprintf(\"%%d\", *req.%s)", f.Name)
	case "bool":
		return fmt.Sprintf("fmt.Sprintf(\"%%t\", *req.%s)", f.Name)
	case "float32", "float64":
		return fmt.Sprintf("fmt.Sprintf(\"%%g\", *req.%s)", f.Name)
	default:
		return fmt.Sprintf("fmt.Sprintf(\"%%v\", *req.%s)", f.Name)
	}
}

func clientRspType(op OperationDef) string {
	switch op.RspTypeName {
	case "struct{}", "ginx.RedirectRsp":
		return ""
	case "ginx.FileRsp":
		return "[]byte"
	case "ginx.StringRsp":
		return "string"
	default:
		return "*" + op.RspTypeName
	}
}

func clientRspSignature(op OperationDef) string {
	rspType := clientRspType(op)
	if rspType == "" {
		return "error"
	}
	return "(" + rspType + ", error)"
}

func skipForClient(op OperationDef) bool {
	if op.Request != nil {
		for _, f := range op.Request.Fields {
			if strings.Contains(f.Type, "multipart.FileHeader") {
				return true
			}
		}
	}
	return false
}

func needsResult(op OperationDef) bool {
	switch op.RspTypeName {
	case "struct{}", "ginx.RedirectRsp", "ginx.FileRsp", "ginx.StringRsp":
		return false
	default:
		return true
	}
}

func isFileRsp(op OperationDef) bool {
	return op.RspTypeName == "ginx.FileRsp"
}

func isStringRsp(op OperationDef) bool {
	return op.RspTypeName == "ginx.StringRsp"
}

func hasSSEOps(ops []OperationDef) bool {
	for _, op := range ops {
		if op.IsSSE {
			return true
		}
	}
	return false
}

func zeroReturn(op OperationDef) string {
	switch clientRspType(op) {
	case "":
		return ""
	case "[]byte":
		return "nil, "
	case "string":
		return "\"\", "
	default:
		return "nil, "
	}
}

func successReturn(op OperationDef) string {
	switch op.RspTypeName {
	case "struct{}", "ginx.RedirectRsp":
		return "nil"
	case "ginx.FileRsp":
		return "resp.Bytes(), nil"
	case "ginx.StringRsp":
		return "resp.String(), nil"
	default:
		return "&result, nil"
	}
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
	GenerateClient    bool
	ServerName        string
}

type clientTemplateData struct {
	PackageName       string
	GenerateDirective string
	Imports           []string
	Operations        []OperationDef
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

func executeClientTemplate(data *clientTemplateData) (string, error) {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "client.go.tmpl", data); err != nil {
		return "", fmt.Errorf("execute client template: %w", err)
	}
	return buf.String(), nil
}
