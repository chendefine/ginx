package codegen

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type OperationDef struct {
	Name        string
	Method      string
	Path        string
	GinPath     string
	IsSSE       bool
	RspTypeName string
	Request     *StructDef
	Response    *TypeDef
}

func ExtractOperations(spec *openapi3.T, cfg *Config, imports map[string]bool, seen map[string]bool) ([]OperationDef, []TypeDef, error) {
	var ops []OperationDef
	var extraTypes []TypeDef

	paths := sortedPaths(spec.Paths.Map())
	for _, path := range paths {
		pathItem := spec.Paths.Map()[path]
		methodOps, err := methodOperations(pathItem)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, mo := range methodOps {
			if mo.op == nil {
				continue
			}

			if cfg != nil && !cfg.ShouldIncludeOperation(mo.op.Tags) {
				continue
			}

			opName := OperationName(mo.method, path, mo.op.OperationID)
			reqStruct, reqExtra := buildRequestStruct(opName, pathItem, mo.op, imports, seen)
			extraTypes = append(extraTypes, reqExtra...)

			sse := isSSEOperation(mo.op)
			rspTypeName := "struct{}"

			var rspDef *TypeDef
			var rspExtra []TypeDef
			if !sse {
				rspTypeName = resolveResponseTypeName(opName, mo.op)
				rspDef, rspExtra = buildResponseType(opName, mo.op, imports, seen)
				extraTypes = append(extraTypes, rspExtra...)
			}

			ops = append(ops, OperationDef{
				Name:        opName,
				Method:      mo.method,
				Path:        path,
				GinPath:     swaggerPathToGin(path),
				IsSSE:       sse,
				RspTypeName: rspTypeName,
				Request:     reqStruct,
				Response:    rspDef,
			})
		}
	}
	return ops, extraTypes, nil
}

func buildRequestStruct(opName string, pathItem *openapi3.PathItem, op *openapi3.Operation, imports map[string]bool, seen map[string]bool) (*StructDef, []TypeDef) {
	reqName := opName + "Req"
	var fields []FieldDef
	var embeds []string
	var extraTypes []TypeDef
	var bodyContentType string

	allParams := mergeParams(pathItem.Parameters, op.Parameters)
	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		fieldName := ToCamelCase(param.Name)
		fieldType := resolveParamType(param, imports)
		required := param.Required

		if param.In == "path" {
			required = true
		}

		var source string
		if param.In == "query" {
			source = fieldSourceQuery
		}
		if !required && !isNilable(fieldType) {
			fieldType = "*" + fieldType
		}

		var tags []Tag
		switch param.In {
		case "path":
			tags = append(tags, Tag{Key: "uri", Value: param.Name})
		case "query":
			tags = append(tags, Tag{Key: "form", Value: param.Name})
		case "header":
			tags = append(tags, Tag{Key: "header", Value: param.Name})
		case "cookie":
			tags = append(tags, Tag{Key: "cookie", Value: param.Name})
		}
		if binding := buildBindingRules(required, param.Schema); binding != "" {
			tags = append(tags, Tag{Key: "binding", Value: binding})
		}
		if param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Default != nil {
			tags = append(tags, Tag{Key: "default", Value: fmt.Sprintf("%v", param.Schema.Value.Default)})
		}

		fields = append(fields, FieldDef{
			Name:   fieldName,
			Type:   fieldType,
			Tags:   tags,
			Source: source,
		})
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		body := op.RequestBody.Value

		if mt := body.Content.Get("multipart/form-data"); mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
			bodyContentType = "multipart/form-data"
			formFields, formExtra := buildFormDataFields(reqName, mt.Schema, imports, seen, true)
			fields = append(fields, formFields...)
			extraTypes = append(extraTypes, formExtra...)
		} else if mt := body.Content.Get("application/json"); mt != nil && mt.Schema != nil {
			bodyContentType = "application/json"
			if mt.Schema.Ref != "" {
				embeds = append(embeds, refToTypeName(mt.Schema.Ref))
			} else if mt.Schema.Value != nil {
				schema := mt.Schema.Value
				if (schema.Type != nil && schema.Type.Is("object")) || len(schema.Properties) > 0 {
					bodyFields, bodyExtra := flattenBodyFields(reqName, mt.Schema, imports, seen)
					fields = append(fields, bodyFields...)
					extraTypes = append(extraTypes, bodyExtra...)
				} else {
					bodyType, extra := resolveFieldType(reqName+"Body", mt.Schema, imports, seen)
					extraTypes = append(extraTypes, extra...)
					tags := []Tag{{Key: "json", Value: "body"}}
					if body.Required {
						tags = append(tags, Tag{Key: "binding", Value: "required"})
					}
					fields = append(fields, FieldDef{
						Name:   "Body",
						Type:   bodyType,
						Tags:   tags,
						Source: fieldSourceBody,
					})
				}
			}
		} else if mt := body.Content.Get("application/x-www-form-urlencoded"); mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
			bodyContentType = "application/x-www-form-urlencoded"
			formFields, formExtra := buildFormDataFields(reqName, mt.Schema, imports, seen, false)
			fields = append(fields, formFields...)
			extraTypes = append(extraTypes, formExtra...)
		}
	}

	if len(fields) == 0 && len(embeds) == 0 {
		return &StructDef{Name: reqName}, nil
	}

	return &StructDef{
		Name:            reqName,
		Fields:          fields,
		Embeds:          embeds,
		BodyContentType: bodyContentType,
	}, extraTypes
}

func flattenBodyFields(parentName string, schemaRef *openapi3.SchemaRef, imports map[string]bool, seen map[string]bool) ([]FieldDef, []TypeDef) {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil, nil
	}
	schema := schemaRef.Value

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var fields []FieldDef
	var extraTypes []TypeDef

	propNames := sortedPropertyNames(schema.Properties)
	for _, propName := range propNames {
		propRef := schema.Properties[propName]
		fieldName := ToCamelCase(propName)
		fieldType, extra := resolveFieldType(parentName+fieldName, propRef, imports, seen)
		extraTypes = append(extraTypes, extra...)

		required := requiredSet[propName]
		if !required && !isNilable(fieldType) {
			fieldType = "*" + fieldType
		}

		tags := []Tag{{Key: "json", Value: propName}}
		if binding := buildBindingRules(required, propRef); binding != "" {
			tags = append(tags, Tag{Key: "binding", Value: binding})
		}
		if propRef != nil && propRef.Value != nil && propRef.Value.Default != nil {
			tags = append(tags, Tag{Key: "default", Value: fmt.Sprintf("%v", propRef.Value.Default)})
		}

		fields = append(fields, FieldDef{
			Name:   fieldName,
			Type:   fieldType,
			Tags:   tags,
			Source: fieldSourceBody,
		})
	}
	return fields, extraTypes
}

func buildFormDataFields(parentName string, schemaRef *openapi3.SchemaRef, imports map[string]bool, seen map[string]bool, allowFiles bool) ([]FieldDef, []TypeDef) {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil, nil
	}
	schema := schemaRef.Value

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var fields []FieldDef
	var extraTypes []TypeDef

	propNames := sortedPropertyNames(schema.Properties)
	for _, propName := range propNames {
		propRef := schema.Properties[propName]
		fieldName := ToCamelCase(propName)

		var fieldType string
		if allowFiles && isFileField(propRef) {
			fieldType = "*multipart.FileHeader"
			imports["mime/multipart"] = true
		} else if allowFiles && isFileArrayField(propRef) {
			fieldType = "[]*multipart.FileHeader"
			imports["mime/multipart"] = true
		} else {
			var extra []TypeDef
			fieldType, extra = resolveFieldType(parentName+fieldName, propRef, imports, seen)
			extraTypes = append(extraTypes, extra...)
		}

		required := requiredSet[propName]
		if !required && !isNilable(fieldType) && !isPointer(fieldType) {
			fieldType = "*" + fieldType
		}

		tags := []Tag{{Key: "form", Value: propName}}
		if binding := buildBindingRules(required, propRef); binding != "" {
			tags = append(tags, Tag{Key: "binding", Value: binding})
		}
		if propRef != nil && propRef.Value != nil && propRef.Value.Default != nil {
			tags = append(tags, Tag{Key: "default", Value: fmt.Sprintf("%v", propRef.Value.Default)})
		}

		fields = append(fields, FieldDef{
			Name:   fieldName,
			Type:   fieldType,
			Tags:   tags,
			Source: fieldSourceBody,
		})
	}
	return fields, extraTypes
}

func isFileField(ref *openapi3.SchemaRef) bool {
	if ref == nil || ref.Value == nil {
		return false
	}
	s := ref.Value
	return s.Type != nil && s.Type.Is("string") && s.Format == "binary"
}

func isFileArrayField(ref *openapi3.SchemaRef) bool {
	if ref == nil || ref.Value == nil {
		return false
	}
	s := ref.Value
	if s.Type != nil && s.Type.Is("array") && s.Items != nil {
		return isFileField(s.Items)
	}
	return false
}

func isPointer(goType string) bool {
	return len(goType) > 0 && goType[0] == '*'
}

func resolveParamType(param *openapi3.Parameter, imports map[string]bool) string {
	if param.Schema == nil {
		return "string"
	}
	return resolveTypeString(param.Schema, imports)
}

func mergeParams(pathParams, opParams openapi3.Parameters) openapi3.Parameters {
	nameMap := make(map[string]bool)
	var result openapi3.Parameters
	for _, p := range opParams {
		if p.Value != nil {
			nameMap[p.Value.In+":"+p.Value.Name] = true
		}
		result = append(result, p)
	}
	for _, p := range pathParams {
		if p.Value != nil {
			key := p.Value.In + ":" + p.Value.Name
			if !nameMap[key] {
				result = append(result, p)
			}
		}
	}
	return result
}

type methodOp struct {
	method string
	op     *openapi3.Operation
}

func methodOperations(item *openapi3.PathItem) ([]methodOp, error) {
	var ops []methodOp
	if item.Get != nil {
		ops = append(ops, methodOp{"GET", item.Get})
	}
	if item.Head != nil {
		ops = append(ops, methodOp{"HEAD", item.Head})
	}
	if item.Post != nil {
		ops = append(ops, methodOp{"POST", item.Post})
	}
	if item.Put != nil {
		ops = append(ops, methodOp{"PUT", item.Put})
	}
	if item.Patch != nil {
		ops = append(ops, methodOp{"PATCH", item.Patch})
	}
	if item.Delete != nil {
		ops = append(ops, methodOp{"DELETE", item.Delete})
	}
	if item.Options != nil {
		ops = append(ops, methodOp{"OPTIONS", item.Options})
	}
	if item.Trace != nil {
		return nil, fmt.Errorf("TRACE operations are not supported by oapi-ginx")
	}
	return ops, nil
}

func sortedPaths(m map[string]*openapi3.PathItem) []string {
	paths := make([]string, 0, len(m))
	for k := range m {
		paths = append(paths, k)
	}
	sort.Strings(paths)
	return paths
}

func swaggerPathToGin(path string) string {
	var result []byte
	i := 0
	for i < len(path) {
		if path[i] == '{' {
			result = append(result, ':')
			i++
			for i < len(path) && path[i] != '}' {
				result = append(result, path[i])
				i++
			}
			if i < len(path) {
				i++ // skip '}'
			}
		} else {
			result = append(result, path[i])
			i++
		}
	}
	return string(result)
}

func isSSEOperation(op *openapi3.Operation) bool {
	if v, ok := op.Extensions["x-ginx-sse"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	if op.Responses == nil {
		return false
	}
	if _, r := selectSuccessResponse(op.Responses); r != nil && r.Value != nil {
		return r.Value.Content.Get("text/event-stream") != nil
	}
	return false
}

func buildResponseType(opName string, op *openapi3.Operation, imports map[string]bool, seen map[string]bool) (*TypeDef, []TypeDef) {
	if op.Responses == nil {
		return nil, nil
	}

	statusCode, responseRef := selectSuccessResponse(op.Responses)
	if responseRef == nil || responseRef.Value == nil {
		return nil, nil
	}
	if statusCode == http.StatusNoContent {
		return nil, nil
	}

	mt := responseRef.Value.Content.Get("application/json")
	if mt == nil || mt.Schema == nil {
		return nil, nil
	}

	rspName := opName + "Rsp"

	if mt.Schema.Ref != "" {
		typeName := refToTypeName(mt.Schema.Ref)
		return &TypeDef{Alias: &AliasDef{Name: rspName, TargetType: typeName}}, nil
	}

	types := ResolveSchema(rspName, mt.Schema, imports, seen)
	if len(types) == 0 {
		return nil, nil
	}
	first := types[0]
	return &first, types[1:]
}

func resolveResponseTypeName(opName string, op *openapi3.Operation) string {
	if op.Responses == nil {
		return "struct{}"
	}

	if !has2xxResponse(op.Responses) && has3xxResponse(op.Responses) {
		return "ginx.RedirectRsp"
	}

	statusCode, responseRef := selectSuccessResponse(op.Responses)
	if responseRef == nil || responseRef.Value == nil {
		return "struct{}"
	}
	if statusCode == http.StatusNoContent {
		return "struct{}"
	}

	content := responseRef.Value.Content
	if content == nil || len(content) == 0 {
		return "struct{}"
	}

	// application/json takes priority → use the generated Rsp type
	if mt := content.Get("application/json"); mt != nil && mt.Schema != nil {
		return opName + "Rsp"
	}

	// check all content types for binary/text classification
	for contentType := range content {
		if isBinaryContentType(contentType) {
			return "ginx.FileRsp"
		}
	}
	for contentType := range content {
		if isTextContentType(contentType) {
			return "ginx.StringRsp"
		}
	}

	return "struct{}"
}

func selectSuccessResponse(responses *openapi3.Responses) (int, *openapi3.ResponseRef) {
	if responses == nil {
		return 0, nil
	}
	for code := http.StatusOK; code < http.StatusMultipleChoices; code++ {
		if r := responses.Status(code); r != nil {
			return code, r
		}
	}
	return 0, nil
}

func has2xxResponse(responses *openapi3.Responses) bool {
	_, r := selectSuccessResponse(responses)
	return r != nil
}

func has3xxResponse(responses *openapi3.Responses) bool {
	if responses == nil {
		return false
	}
	for name := range responses.Map() {
		code, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		if code >= http.StatusMultipleChoices && code < http.StatusBadRequest {
			return true
		}
	}
	return false
}

func isBinaryContentType(ct string) bool {
	switch ct {
	case "application/octet-stream", "application/pdf", "application/zip",
		"application/gzip", "application/x-tar", "application/x-gzip":
		return true
	}
	if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		return true
	}
	if strings.HasPrefix(ct, "application/") {
		if ct == "application/json" || ct == "application/xml" ||
			ct == "application/x-www-form-urlencoded" ||
			ct == "application/graphql" ||
			strings.HasSuffix(ct, "+json") ||
			strings.HasSuffix(ct, "+xml") {
			return false
		}
		return true
	}
	return false
}

func isTextContentType(ct string) bool {
	return strings.HasPrefix(ct, "text/")
}
