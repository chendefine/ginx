package codegen

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type OperationDef struct {
	Name             string
	Method           string
	Path             string
	GinPath          string
	IsSSE            bool
	IsJSONLines      bool
	IsNoBody         bool
	SuccessStatus    int
	ExpectedStatuses []int
	ResponseMode     string
	RspTypeName      string
	Request          *StructDef
	Response         *TypeDef
	ResponseVariants []ResponseVariantDef
}

type ResponseVariantDef struct {
	StatusCode int
	TypeName   string
	HasBody    bool
}

func ExtractOperations(spec *openapi3.T, cfg *Config, imports map[string]bool, seen map[string]bool) ([]OperationDef, []TypeDef, error) {
	var ops []OperationDef
	var extraTypes []TypeDef

	for _, path := range sortedPaths(spec.Paths.Map()) {
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
			opDef, extra, err := buildOperationDef(mo.method, path, pathItem, mo.op, cfg, imports, seen)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", path, err)
			}
			ops = append(ops, opDef)
			extraTypes = append(extraTypes, extra...)
		}
	}

	// Webhooks (OpenAPI 3.1+): each entry is an inbound operation the server
	// must handle. The webhook name is an identifier, not a URL — synthesize a
	// deterministic /webhooks/<name> route. Walk keys alphabetically for
	// reproducible output. Absent in 3.0 specs, so this is a no-op there.
	for _, whName := range sortedWebhookNames(spec.Webhooks) {
		pathItem := spec.Webhooks[whName]
		if pathItem == nil {
			continue
		}
		methodOps, err := methodOperations(pathItem)
		if err != nil {
			return nil, nil, fmt.Errorf("webhook %s: %w", whName, err)
		}
		whPath := "/webhooks/" + sanitizePathSegment(whName)
		for _, mo := range methodOps {
			if mo.op == nil {
				continue
			}
			if cfg != nil && !cfg.ShouldIncludeOperation(mo.op.Tags) {
				continue
			}
			opDef, extra, err := buildOperationDef(mo.method, whPath, pathItem, mo.op, cfg, imports, seen)
			if err != nil {
				return nil, nil, fmt.Errorf("webhook %s: %w", whName, err)
			}
			ops = append(ops, opDef)
			extraTypes = append(extraTypes, extra...)
		}
	}

	return ops, extraTypes, nil
}

// buildOperationDef turns a single (method, path, operation) into an
// OperationDef plus any auxiliary types (request body flattening, response
// schema). Shared by the paths loop and the webhooks loop so the two stay in
// lockstep.
func buildOperationDef(method, path string, pathItem *openapi3.PathItem, op *openapi3.Operation, cfg *Config, imports map[string]bool, seen map[string]bool) (OperationDef, []TypeDef, error) {
	opName := OperationName(method, path, op.OperationID)
	responseMode, err := operationResponseMode(op)
	if err != nil {
		return OperationDef{}, nil, fmt.Errorf("%s %s (%s): %w", method, path, opName, err)
	}
	if responseMode == "variants" {
		if err := validateVariantResponses(opName, method, path, op); err != nil {
			return OperationDef{}, nil, err
		}
	} else if err := validateOperationResponses(opName, method, path, op); err != nil {
		return OperationDef{}, nil, err
	}
	reqStruct, reqExtra := buildRequestStruct(opName, pathItem, op, imports, seen)

	sse := isSSEOperation(op)
	jl := isJSONLinesOperation(op)
	successStatus, _ := selectSuccessResponse(op.Responses)
	if successStatus == 0 && !has2xxResponse(op.Responses) {
		successStatus = firstRedirectStatus(op.Responses)
	}
	if sse {
		if err := validateStreamingSuccessStatuses("SSE", opName, method, path, op.Responses, successStatus); err != nil {
			return OperationDef{}, nil, err
		}
	}
	if jl {
		if err := validateStreamingSuccessStatuses("JSON Lines", opName, method, path, op.Responses, successStatus); err != nil {
			return OperationDef{}, nil, err
		}
	}
	if responseMode == "variants" && (sse || jl) {
		return OperationDef{}, nil, fmt.Errorf("%s %s (%s): x-ginx-response-mode=variants does not support streaming responses", method, path, opName)
	}
	rspTypeName := "struct{}"

	var rspDef *TypeDef
	var rspExtra []TypeDef
	var variants []ResponseVariantDef
	if responseMode == "variants" {
		rspTypeName = opName + "Response"
		variants, rspExtra = buildResponseVariants(opName, op, cfg.ShouldUnwrapEnvelope(), imports, seen)
		successStatus = 0
	} else if !sse && !jl {
		rspTypeName = resolveResponseTypeName(opName, op)
		if err := validateFileResponseContract(opName, method, path, op, rspTypeName); err != nil {
			return OperationDef{}, nil, err
		}
		rspDef, rspExtra = buildResponseType(opName, op, cfg.ShouldUnwrapEnvelope(), imports, seen)
	}
	expectedStatuses := expectedResponseStatuses(op, successStatus)
	if responseMode == "variants" {
		expectedStatuses = variantStatusCodes(op.Responses)
	}

	return OperationDef{
		Name:             opName,
		Method:           method,
		Path:             path,
		GinPath:          swaggerPathToGin(path),
		IsSSE:            sse,
		IsJSONLines:      jl,
		IsNoBody:         responseMode != "variants" && (strings.EqualFold(method, http.MethodHead) || successStatus == http.StatusNoContent),
		SuccessStatus:    successStatus,
		ExpectedStatuses: expectedStatuses,
		ResponseMode:     responseMode,
		RspTypeName:      rspTypeName,
		Request:          reqStruct,
		Response:         rspDef,
		ResponseVariants: variants,
	}, append(reqExtra, rspExtra...), nil
}

func buildRequestStruct(opName string, pathItem *openapi3.PathItem, op *openapi3.Operation, imports map[string]bool, seen map[string]bool) (*StructDef, []TypeDef) {
	reqName := opName + "Req"
	var fields []FieldDef
	var embeds []string
	var extraTypes []TypeDef
	var bodyContentType string
	var aliasTarget string

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
		// OpenAPI 3.2 introduces `in: querystring` (bind the whole query string
		// as one schema). kin-openapi drops the structured form, leaving a flat
		// scalar Parameter, so we treat it like an ordinary query parameter.
		if param.In == "query" || param.In == "querystring" {
			source = fieldSourceQuery
		}
		if !required && !isNilable(fieldType) {
			fieldType = "*" + fieldType
		}

		var tags []Tag
		switch param.In {
		case "path":
			tags = append(tags, Tag{Key: "uri", Value: param.Name})
		case "query", "querystring":
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
				refType := refToTypeName(mt.Schema.Ref)
				if len(fields) == 0 {
					aliasTarget = refType
				} else {
					embeds = append(embeds, refType)
				}
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

	if len(fields) == 0 && len(embeds) == 0 && aliasTarget == "" {
		return &StructDef{Name: reqName}, nil
	}

	return &StructDef{
		Name:            reqName,
		Fields:          fields,
		Embeds:          embeds,
		AliasTarget:     aliasTarget,
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

func sortedWebhookNames(m map[string]*openapi3.PathItem) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// sanitizePathSegment produces a gin-route-safe path segment from a webhook
// name. Webhook names are spec-defined identifiers (often CamelCase or
// kebab-case), not URLs; lower-case and replace any byte outside [a-z0-9._-]
// with '-' so the generated route never conflicts with gin's path grammar.
func sanitizePathSegment(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
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

// isJSONLinesOperation detects NDJSON / JSON Lines streaming responses. The
// x-ginx-jsonl extension (mirroring x-ginx-sse) lets a spec declare streaming
// even when the media type is application/json; otherwise the success response
// media type must be application/jsonl or application/x-ndjson.
//
// application/json-seq (RFC 7464) is intentionally EXCLUDED: its framing
// prepends a 0x1E record separator before each value, which would corrupt the
// newline-splitting reader in ginx.JSONLinesStream.
func isJSONLinesOperation(op *openapi3.Operation) bool {
	if v, ok := op.Extensions["x-ginx-jsonl"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	if op.Responses == nil {
		return false
	}
	if _, r := selectSuccessResponse(op.Responses); r != nil && r.Value != nil {
		content := r.Value.Content
		for contentType, mediaType := range content {
			if mediaType != nil && isJSONLinesContentType(contentType) {
				return true
			}
		}
	}
	return false
}

func isJSONLinesContentType(ct string) bool {
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "application/jsonl") ||
		strings.EqualFold(mediaType, "application/x-ndjson")
}

func operationResponseMode(op *openapi3.Operation) (string, error) {
	if op == nil {
		return "simple", nil
	}
	v, ok := op.Extensions["x-ginx-response-mode"]
	if !ok {
		return "simple", nil
	}
	mode, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("x-ginx-response-mode must be \"variants\"")
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "simple", "":
		return "simple", nil
	case "variants":
		return "variants", nil
	default:
		return "", fmt.Errorf("x-ginx-response-mode=%q is unsupported; use variants", mode)
	}
}

func buildResponseVariants(opName string, op *openapi3.Operation, unwrap bool, imports map[string]bool, seen map[string]bool) ([]ResponseVariantDef, []TypeDef) {
	if op == nil || op.Responses == nil {
		return nil, nil
	}
	var variants []ResponseVariantDef
	var types []TypeDef
	for _, status := range variantStatusCodes(op.Responses) {
		responseRef := op.Responses.Status(status)
		variant := ResponseVariantDef{StatusCode: status}
		if responseRef != nil && responseRef.Value != nil {
			if mt := responseRef.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil {
				variant.HasBody = true
				variant.TypeName = fmt.Sprintf("%s%dRsp", opName, status)
				bodyTypes := buildNamedResponseType(variant.TypeName, mt.Schema, unwrap, imports, seen)
				types = append(types, bodyTypes...)
			}
		}
		variants = append(variants, variant)
	}
	return variants, types
}

func buildNamedResponseType(typeName string, schema *openapi3.SchemaRef, unwrap bool, imports map[string]bool, seen map[string]bool) []TypeDef {
	effective := effectiveResponseSchema(schema, unwrap)
	if effective == nil {
		return nil
	}
	if effective.Ref != "" {
		return []TypeDef{{Alias: &AliasDef{Name: typeName, TargetType: refToTypeName(effective.Ref)}}}
	}
	return ResolveSchema(typeName, effective, imports, seen)
}

func buildResponseType(opName string, op *openapi3.Operation, unwrap bool, imports map[string]bool, seen map[string]bool) (*TypeDef, []TypeDef) {
	if op.Responses == nil {
		return nil, nil
	}

	statusCode, responseRef := selectSuccessResponse(op.Responses)
	if responseRef == nil || responseRef.Value == nil {
		return nil, nil
	}
	if kind := selectedResponseOverride(op, responseRef); kind != "" {
		return nil, nil
	}
	if statusCode == http.StatusNoContent {
		return nil, nil
	}

	mt := responseRef.Value.Content.Get("application/json")
	if mt == nil || mt.Schema == nil {
		return nil, nil
	}

	// When unwrap is enabled and the response schema is exactly the ginx success
	// envelope {code,msg,data}, derive XxxRsp from the data sub-schema only.
	// This keeps the generated type as the business payload and lets ginx's
	// runtime success wrapper add the single envelope, avoiding a double-wrapped
	// wire body. effective keeps its own .Ref/.Value so the alias/ResolveSchema
	// branches below work unchanged.
	rspName := opName + "Rsp"
	types := buildNamedResponseType(rspName, mt.Schema, unwrap, imports, seen)
	if len(types) == 0 {
		return nil, nil
	}
	first := types[0]
	return &first, types[1:]
}

// effectiveResponseSchema returns the schema to generate the XxxRsp type from.
// When unwrap is enabled and schema (inline or $ref-resolved) is exactly the
// ginx success envelope {code:integer, msg:string, data:any}, it returns the
// data sub-schema; otherwise it returns schema unchanged. The returned ref
// retains its own .Ref/.Value so callers' alias-vs-ResolveSchema logic works
// without further changes.
//
// Besides the directly-written envelope, this also recognizes the common
// reusable-envelope pattern where the response is an allOf that composes a
// generic Envelope component with a specific data override:
//
//	allOf:
//	  - $ref: '#/components/schemas/Envelope'   # {code,msg,data:<generic>}
//	  - properties:
//	      data:
//	        $ref: '#/components/schemas/UserProfile'
func effectiveResponseSchema(ref *openapi3.SchemaRef, unwrap bool) *openapi3.SchemaRef {
	if !unwrap || ref == nil || ref.Value == nil {
		return ref
	}
	s := ref.Value
	if isGinxEnvelope(s) {
		if data := s.Properties["data"]; data != nil {
			return data
		}
	}
	if len(s.AllOf) > 0 {
		if data := dataFromAllOfEnvelope(s); data != nil {
			return data
		}
	}
	return ref
}

// isGinxEnvelope reports whether s is exactly ginx's default success envelope:
// precisely three properties code/msg/data, with integer code and string msg.
// The required array is ignored (ginx always emits all three fields).
func isGinxEnvelope(s *openapi3.Schema) bool {
	if s == nil || len(s.Properties) != 3 {
		return false
	}
	code, msg, data := s.Properties["code"], s.Properties["msg"], s.Properties["data"]
	if code == nil || code.Value == nil || msg == nil || msg.Value == nil || data == nil {
		return false
	}
	return typeIs(code.Value, "integer") && typeIs(msg.Value, "string")
}

// dataFromAllOfEnvelope flattens s by merging its own properties with those of
// every allOf member (recursively, $ref members resolved to their .Value). If
// the merged shape is exactly ginx's success envelope {code:integer,
// msg:string, data:any}, it returns the merged data sub-schema; otherwise nil.
// This is what makes the reusable-envelope pattern unwrap correctly:
//
//	allOf:
//	  - $ref: '#/components/schemas/Envelope'   # {code,msg,data:<generic>}
//	  - properties:
//	      data:
//	        $ref: '#/components/schemas/UserProfile'
//
// A nil return lets the caller fall back to the original schema (and the
// existing resolveAllOf path), so non-envelope allOf schemas are unaffected.
func dataFromAllOfEnvelope(s *openapi3.Schema) *openapi3.SchemaRef {
	merged := flattenAllOf(s)
	if !isGinxEnvelope(merged) {
		return nil
	}
	return merged.Properties["data"]
}

// flattenAllOf merges s and all of its allOf members into a single schema,
// combining properties via mergeProperty (more concrete definitions win,
// independent of member order). It exists only to evaluate the envelope shape,
// so it deliberately ignores $ref identity/embedding semantics. A *Schema
// pointer visited set guards against self-referential allOf cycles.
func flattenAllOf(s *openapi3.Schema) *openapi3.Schema {
	merged := &openapi3.Schema{Properties: make(openapi3.Schemas)}
	flattenInto(merged, s, map[*openapi3.Schema]bool{})
	return merged
}

func flattenInto(dst, src *openapi3.Schema, visited map[*openapi3.Schema]bool) {
	if src == nil || visited[src] {
		return
	}
	visited[src] = true
	for k, v := range src.Properties {
		mergeProperty(dst, k, v)
	}
	for _, member := range src.AllOf {
		if member != nil && member.Value != nil {
			flattenInto(dst, member.Value, visited)
		}
	}
}

// mergeProperty merges a single property: a concrete definition overrides a
// placeholder one, and ties resolve last-wins. This keeps a specific data
// override from being clobbered by the generic envelope's data regardless of
// allOf member order (reversed order would otherwise silently unwrap to an
// empty/generic type).
func mergeProperty(dst *openapi3.Schema, k string, v *openapi3.SchemaRef) {
	existing, ok := dst.Properties[k]
	if !ok {
		dst.Properties[k] = v
		return
	}
	if schemaRefHasContent(existing) && !schemaRefHasContent(v) {
		return // existing is the concrete definition, keep it
	}
	dst.Properties[k] = v
}

// schemaRefHasContent reports whether ref carries a real type definition (as
// opposed to a placeholder like the envelope's generic data that only has a
// description).
func schemaRefHasContent(ref *openapi3.SchemaRef) bool {
	if ref == nil {
		return false
	}
	if ref.Ref != "" {
		return true
	}
	v := ref.Value
	return v != nil && (v.Type != nil && !v.Type.IsEmpty() || v.Format != "" || len(v.Properties) > 0 ||
		v.Items != nil || v.AdditionalProperties.Schema != nil || v.Enum != nil ||
		len(v.AllOf) > 0 || len(v.OneOf) > 0 || len(v.AnyOf) > 0)
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
	if kind := selectedResponseOverride(op, responseRef); kind != "" {
		return responseKindType(kind)
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
	for _, code := range successStatusCodes(responses) {
		r := responses.Status(code)
		if isPrimarySuccessResponse(r) {
			return code, r
		}
	}
	for code := http.StatusOK; code < http.StatusMultipleChoices; code++ {
		if r := responses.Status(code); r != nil {
			return code, r
		}
	}
	return 0, nil
}

func validateOperationResponses(opName, method, path string, op *openapi3.Operation) error {
	if op == nil {
		return nil
	}
	if _, _, err := responseOverrideFromExtensions(op.Extensions); err != nil {
		return fmt.Errorf("%s %s (%s): %w", method, path, opName, err)
	}
	if op.Responses == nil {
		return nil
	}

	hasPrimary := false
	primaryCount := 0
	for _, code := range successStatusCodes(op.Responses) {
		responseRef := op.Responses.Status(code)
		if isPrimarySuccessResponse(responseRef) {
			primaryCount++
			hasPrimary = true
		}
	}
	if primaryCount > 1 {
		return fmt.Errorf("%s %s (%s): multiple 2xx responses set x-ginx-primary-response: true; keep only one primary success response", method, path, opName)
	}

	var firstJSONStatus int
	var firstJSONFingerprint string
	for _, code := range successStatusCodes(op.Responses) {
		responseRef := op.Responses.Status(code)
		if responseRef == nil || responseRef.Value == nil {
			continue
		}
		if _, _, err := responseOverrideFromExtensions(responseRef.Value.Extensions); err != nil {
			return fmt.Errorf("%s %s (%s) response %d: %w", method, path, opName, code, err)
		}
		mt := responseRef.Value.Content.Get("application/json")
		if mt == nil || mt.Schema == nil {
			continue
		}
		fp := responseSchemaFingerprint(mt.Schema)
		if firstJSONFingerprint == "" {
			firstJSONStatus = code
			firstJSONFingerprint = fp
			continue
		}
		if !hasPrimary && fp != firstJSONFingerprint {
			return fmt.Errorf("%s %s (%s): multiple 2xx JSON responses have different schemas (%d and %d); keep one JSON success schema or set x-ginx-primary-response: true on the intended primary response", method, path, opName, firstJSONStatus, code)
		}
	}
	return nil
}

func validateVariantResponses(opName, method, path string, op *openapi3.Operation) error {
	prefix := fmt.Sprintf("%s %s (%s): x-ginx-response-mode=variants", method, path, opName)
	if op == nil || op.Responses == nil {
		return fmt.Errorf("%s requires at least one explicit numeric 2xx or 3xx response", prefix)
	}
	if isSSEOperation(op) || isJSONLinesOperation(op) {
		return fmt.Errorf("%s does not support streaming responses", prefix)
	}
	if _, ok, err := responseOverrideFromExtensions(op.Extensions); err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	} else if ok {
		return fmt.Errorf("%s conflicts with x-ginx-response", prefix)
	}
	for name := range op.Responses.Map() {
		if _, err := strconv.Atoi(name); err != nil {
			return fmt.Errorf("%s does not support wildcard or default response %q", prefix, name)
		}
	}
	statuses := variantStatusCodes(op.Responses)
	if len(statuses) == 0 {
		return fmt.Errorf("%s requires at least one explicit numeric 2xx or 3xx response", prefix)
	}
	for _, status := range statuses {
		responseRef := op.Responses.Status(status)
		if responseRef == nil || responseRef.Value == nil {
			return fmt.Errorf("%s response %d is unresolved", prefix, status)
		}
		response := responseRef.Value
		if isPrimarySuccessResponse(responseRef) {
			return fmt.Errorf("%s conflicts with x-ginx-primary-response on response %d", prefix, status)
		}
		if _, ok, err := responseOverrideFromExtensions(response.Extensions); err != nil {
			return fmt.Errorf("%s response %d: %w", prefix, status, err)
		} else if ok {
			return fmt.Errorf("%s does not support x-ginx-response on response %d", prefix, status)
		}
		if len(response.Headers) > 0 {
			return fmt.Errorf("%s does not support typed response headers on response %d", prefix, status)
		}
		if len(response.Content) == 0 {
			continue
		}
		if len(response.Content) != 1 || response.Content.Get("application/json") == nil {
			return fmt.Errorf("%s response %d supports only application/json or no body", prefix, status)
		}
		mt := response.Content.Get("application/json")
		if mt.Schema == nil {
			return fmt.Errorf("%s response %d application/json content requires a schema", prefix, status)
		}
		if strings.EqualFold(method, http.MethodHead) || status == http.StatusNoContent || status == http.StatusNotModified {
			return fmt.Errorf("%s response %d must not declare a body", prefix, status)
		}
	}
	return nil
}

func validateFileResponseContract(opName, method, path string, op *openapi3.Operation, rspTypeName string) error {
	if rspTypeName != "ginx.FileRsp" || op == nil || op.Responses == nil {
		return nil
	}
	prefix := fmt.Sprintf("%s %s (%s): file response", method, path, opName)
	for _, status := range successStatusCodes(op.Responses) {
		if status != http.StatusOK && status != http.StatusPartialContent {
			return fmt.Errorf("%s does not support success status %d; use 200 and optionally 206", prefix, status)
		}
	}
	partial := op.Responses.Status(http.StatusPartialContent)
	if partial == nil {
		return nil
	}
	complete := op.Responses.Status(http.StatusOK)
	if complete == nil {
		return fmt.Errorf("%s declares 206 without 200; http.ServeFile returns 200 when no Range is requested", prefix)
	}
	if responseContractFingerprint(op, complete) != responseContractFingerprint(op, partial) {
		return fmt.Errorf("%s responses 200 and 206 must use compatible content and schemas", prefix)
	}
	return nil
}

func validateStreamingSuccessStatuses(kind, opName, method, path string, responses *openapi3.Responses, selectedStatus int) error {
	if selectedStatus != http.StatusOK {
		return fmt.Errorf("%s %s (%s): %s success response must use HTTP 200", method, path, opName, kind)
	}
	for _, status := range successStatusCodes(responses) {
		if status != http.StatusOK {
			return fmt.Errorf("%s %s (%s): %s success response must use HTTP 200; response %d is not supported", method, path, opName, kind, status)
		}
	}
	return nil
}

func successStatusCodes(responses *openapi3.Responses) []int {
	return responseStatusCodes(responses, http.StatusOK, http.StatusMultipleChoices)
}

func redirectStatusCodes(responses *openapi3.Responses) []int {
	return responseStatusCodes(responses, http.StatusMultipleChoices, http.StatusBadRequest)
}

func variantStatusCodes(responses *openapi3.Responses) []int {
	return responseStatusCodes(responses, http.StatusOK, http.StatusBadRequest)
}

func responseStatusCodes(responses *openapi3.Responses, min, max int) []int {
	if responses == nil {
		return nil
	}
	var codes []int
	for name := range responses.Map() {
		code, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		if code >= min && code < max {
			codes = append(codes, code)
		}
	}
	sort.Ints(codes)
	return codes
}

func firstRedirectStatus(responses *openapi3.Responses) int {
	codes := redirectStatusCodes(responses)
	if len(codes) == 0 {
		return 0
	}
	return codes[0]
}

func expectedResponseStatuses(op *openapi3.Operation, selectedStatus int) []int {
	if op == nil || op.Responses == nil || selectedStatus == 0 {
		return nil
	}
	if selectedStatus >= http.StatusMultipleChoices {
		return redirectStatusCodes(op.Responses)
	}
	selected := op.Responses.Status(selectedStatus)
	selectedKey := responseContractFingerprint(op, selected)
	var result []int
	for _, code := range successStatusCodes(op.Responses) {
		if responseContractFingerprint(op, op.Responses.Status(code)) == selectedKey {
			result = append(result, code)
		}
	}
	if len(result) == 0 {
		return []int{selectedStatus}
	}
	return result
}

func responseContractFingerprint(op *openapi3.Operation, responseRef *openapi3.ResponseRef) string {
	if responseRef == nil || responseRef.Value == nil {
		return "<nil>"
	}
	if kind := selectedResponseOverride(op, responseRef); kind != "" {
		return "override:" + kind
	}
	content := responseRef.Value.Content
	if len(content) == 0 {
		return "empty"
	}
	keys := make([]string, 0, len(content))
	for contentType, mediaType := range content {
		fingerprint := "<nil>"
		if mediaType != nil {
			fingerprint = responseSchemaFingerprint(mediaType.Schema)
		}
		keys = append(keys, contentType+":"+fingerprint)
	}
	sort.Strings(keys)
	return strings.Join(keys, "|")
}

func isPrimarySuccessResponse(responseRef *openapi3.ResponseRef) bool {
	if responseRef == nil || responseRef.Value == nil {
		return false
	}
	v, ok := responseRef.Value.Extensions["x-ginx-primary-response"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func selectedResponseOverride(op *openapi3.Operation, responseRef *openapi3.ResponseRef) string {
	if responseRef != nil && responseRef.Value != nil {
		if kind, ok, _ := responseOverrideFromExtensions(responseRef.Value.Extensions); ok {
			return kind
		}
	}
	if op != nil {
		if kind, ok, _ := responseOverrideFromExtensions(op.Extensions); ok {
			return kind
		}
	}
	return ""
}

func responseOverrideFromExtensions(extensions map[string]any) (string, bool, error) {
	v, ok := extensions["x-ginx-response"]
	if !ok {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", true, fmt.Errorf("x-ginx-response must be one of file, string, data, redirect")
	}
	kind := strings.ToLower(strings.TrimSpace(s))
	switch kind {
	case "file", "string", "data", "redirect":
		return kind, true, nil
	default:
		return "", true, fmt.Errorf("x-ginx-response=%q is unsupported; use file, string, data, or redirect", s)
	}
}

func responseKindType(kind string) string {
	switch kind {
	case "file":
		return "ginx.FileRsp"
	case "string":
		return "ginx.StringRsp"
	case "data":
		return "ginx.DataRsp"
	case "redirect":
		return "ginx.RedirectRsp"
	default:
		return "struct{}"
	}
}

func responseSchemaFingerprint(schemaRef *openapi3.SchemaRef) string {
	if schemaRef == nil {
		return "<nil>"
	}
	if schemaRef.Ref != "" {
		return "ref:" + schemaRef.Ref
	}
	b, err := json.Marshal(schemaRef.Value)
	if err != nil {
		return fmt.Sprintf("%#v", schemaRef.Value)
	}
	return string(b)
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
			ct == "application/jsonl" || ct == "application/x-ndjson" ||
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
