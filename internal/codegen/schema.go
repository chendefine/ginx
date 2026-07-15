package codegen

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

type Tag struct {
	Key   string
	Value string
}

const (
	fieldSourceQuery = "query"
	fieldSourceBody  = "body"
)

type FieldDef struct {
	Name    string
	Type    string
	Tags    []Tag
	Comment string

	// Source is OpenAPI metadata used by client generation only. It must not
	// change Gin's runtime tag semantics; for example, `form` still binds both
	// query strings and form bodies on the server side.
	Source string
}

type StructDef struct {
	Name            string
	Comment         string
	Fields          []FieldDef
	Embeds          []string
	AliasTarget     string
	BodyContentType string
}

type EnumDef struct {
	TypeName string
	BaseType string
	Values   []EnumValue
	Comment  string
}

type EnumValue struct {
	Name  string
	Value string
}

type TypeDef struct {
	Struct *StructDef
	Enum   *EnumDef
	Alias  *AliasDef
}

type AliasDef struct {
	Name       string
	TargetType string
	Comment    string
}

func ResolveSchema(name string, schemaRef *openapi3.SchemaRef, imports map[string]bool, seen map[string]bool) []TypeDef {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil
	}

	if seen[name] {
		return nil
	}
	seen[name] = true

	schema := schemaRef.Value

	if len(schema.Enum) > 0 {
		return resolveEnum(name, schema)
	}

	if typeIs(schema, "array") {
		return resolveArrayAlias(name, schema, imports)
	}

	if typeIs(schema, "object") || len(schema.Properties) > 0 {
		return resolveObject(name, schema, imports, seen)
	}

	if len(schema.AllOf) > 0 {
		return resolveAllOf(name, schema, imports, seen)
	}

	if len(schema.OneOf) > 0 || len(schema.AnyOf) > 0 {
		imports["encoding/json"] = true
		return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: "json.RawMessage", Comment: schema.Description}}}
	}

	goType, imp := MapType(schemaTypeStr(schema), schema.Format)
	if imp != "" {
		imports[imp] = true
	}
	return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: goType, Comment: schema.Description}}}
}

func resolveEnum(name string, schema *openapi3.Schema) []TypeDef {
	baseType, _ := MapType(schemaTypeStr(schema), schema.Format)
	var values []EnumValue
	for _, v := range schema.Enum {
		valStr := fmt.Sprintf("%v", v)
		constName := name + ToCamelCase(valStr)
		var literal string
		if baseType == "string" {
			literal = fmt.Sprintf("%q", valStr)
		} else {
			literal = valStr
		}
		values = append(values, EnumValue{Name: constName, Value: literal})
	}
	return []TypeDef{{Enum: &EnumDef{TypeName: name, BaseType: baseType, Values: values, Comment: schema.Description}}}
}

func resolveArrayAlias(name string, schema *openapi3.Schema, imports map[string]bool) []TypeDef {
	if isArrayTuple(schema) {
		return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: "[]any", Comment: schema.Description}}}
	}
	elemType := resolveTypeString(schema.Items, imports)
	return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: "[]" + elemType, Comment: schema.Description}}}
}

// isArrayTuple reports whether an array schema is an OpenAPI 3.1 positional
// tuple declared via prefixItems. Such tuples are mapped to []any because JSON
// arrays do not unmarshal into ordinary Go structs without per-tuple custom
// codecs, and uniform-tuple ([N]T) detection adds complexity for a rare case.
// Callers wanting positional typing define a domain struct with custom
// MarshalJSON/UnmarshalJSON.
func isArrayTuple(schema *openapi3.Schema) bool {
	return typeIs(schema, "array") && len(schema.PrefixItems) > 0
}

func resolveObject(name string, schema *openapi3.Schema, imports map[string]bool, seen map[string]bool) []TypeDef {
	if len(schema.Properties) == 0 && schema.AdditionalProperties.Schema != nil {
		valType := resolveTypeString(schema.AdditionalProperties.Schema, imports)
		return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: "map[string]" + valType, Comment: schema.Description}}}
	}

	if len(schema.Properties) == 0 && schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has {
		return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: "map[string]any", Comment: schema.Description}}}
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var fields []FieldDef
	var additionalTypes []TypeDef

	propNames := sortedPropertyNames(schema.Properties)
	for _, propName := range propNames {
		propRef := schema.Properties[propName]
		fieldName := ToCamelCase(propName)
		fieldType, extra := resolveFieldType(name+fieldName, propRef, imports, seen)
		additionalTypes = append(additionalTypes, extra...)

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
			Name:    fieldName,
			Type:    fieldType,
			Tags:    tags,
			Comment: schemaDescription(propRef),
		})
	}

	result := []TypeDef{{Struct: &StructDef{
		Name:    name,
		Comment: schema.Description,
		Fields:  fields,
	}}}
	return append(result, additionalTypes...)
}

func resolveAllOf(name string, schema *openapi3.Schema, imports map[string]bool, seen map[string]bool) []TypeDef {
	merged := &openapi3.Schema{
		Properties:  make(openapi3.Schemas),
		Description: schema.Description,
	}
	merged.Required = append(merged.Required, schema.Required...)

	var embeds []string
	for _, ref := range schema.AllOf {
		if ref.Ref != "" {
			refName := refToTypeName(ref.Ref)
			embeds = append(embeds, refName)
			continue
		}
		if ref.Value == nil {
			continue
		}
		for k, v := range ref.Value.Properties {
			merged.Properties[k] = v
		}
		merged.Required = append(merged.Required, ref.Value.Required...)
		if merged.Description == "" {
			merged.Description = ref.Value.Description
		}
		if merged.AdditionalProperties.Schema == nil {
			merged.AdditionalProperties = ref.Value.AdditionalProperties
		}
	}

	if len(merged.Properties) == 0 && len(embeds) > 0 {
		if len(embeds) == 1 {
			return []TypeDef{{Alias: &AliasDef{Name: name, TargetType: embeds[0], Comment: merged.Description}}}
		}
		return []TypeDef{{Struct: &StructDef{Name: name, Comment: merged.Description, Embeds: embeds}}}
	}

	types := resolveObject(name, merged, imports, seen)
	if len(types) > 0 && types[0].Struct != nil {
		types[0].Struct.Embeds = embeds
	}
	return types
}

func schemaDescription(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil {
		return ""
	}
	return ref.Value.Description
}

func resolveFieldType(nestedName string, ref *openapi3.SchemaRef, imports map[string]bool, seen map[string]bool) (string, []TypeDef) {
	if ref == nil || ref.Ref != "" || ref.Value == nil {
		return resolveTypeString(ref, imports), nil
	}
	schema := ref.Value

	if typeIs(schema, "object") && len(schema.Properties) > 0 {
		types := ResolveSchema(nestedName, ref, imports, seen)
		return nestedName, types
	}

	if typeIs(schema, "array") && schema.Items != nil {
		if isArrayTuple(schema) {
			return "[]any", nil
		}
		elemType, extra := resolveFieldType(nestedName+"Item", schema.Items, imports, seen)
		return "[]" + elemType, extra
	}

	return resolveTypeString(ref, imports), nil
}

func resolveTypeString(ref *openapi3.SchemaRef, imports map[string]bool) string {
	if ref == nil {
		return "any"
	}
	if ref.Ref != "" {
		return refToTypeName(ref.Ref)
	}
	schema := ref.Value
	if schema == nil {
		return "any"
	}

	if typeIs(schema, "array") {
		if isArrayTuple(schema) {
			return "[]any"
		}
		elemType := resolveTypeString(schema.Items, imports)
		return "[]" + elemType
	}

	if typeIs(schema, "object") {
		if schema.AdditionalProperties.Schema != nil {
			valType := resolveTypeString(schema.AdditionalProperties.Schema, imports)
			return "map[string]" + valType
		}
		if len(schema.Properties) == 0 {
			return "map[string]any"
		}
		return "any"
	}

	if len(schema.AllOf) > 0 || len(schema.OneOf) > 0 || len(schema.AnyOf) > 0 {
		imports["encoding/json"] = true
		return "json.RawMessage"
	}

	goType, imp := MapType(schemaTypeStr(schema), schema.Format)
	if imp != "" {
		imports[imp] = true
	}
	return goType
}

func refToTypeName(ref string) string {
	parts := splitRef(ref)
	if len(parts) == 0 {
		return "any"
	}
	return ToIdentifier(parts[len(parts)-1])
}

func splitRef(ref string) []string {
	var parts []string
	for _, p := range splitBySlash(ref) {
		if p != "" && p != "#" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitBySlash(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func isNilable(goType string) bool {
	if len(goType) >= 2 && goType[:2] == "[]" {
		return true
	}
	if len(goType) >= 4 && goType[:4] == "map[" {
		return true
	}
	return false
}

func schemaTypeStr(schema *openapi3.Schema) string {
	if schema.Type == nil {
		return ""
	}
	types := schema.Type.Slice()
	if len(types) == 0 {
		return ""
	}
	return types[0]
}

// typeIs reports whether schema's type set contains typ. Unlike
// (*openapi3.Types).Is (which is strict single-type, returning false for
// OpenAPI 3.1 nullable type arrays like ["array","null"]), this tolerates a
// "null" companion so nullable structured types still resolve to their
// structured Go form (e.g. ["array","null"] -> []string, since Go slices are
// already nilable).
func typeIs(schema *openapi3.Schema, typ string) bool {
	return schema != nil && schema.Type != nil && schema.Type.Includes(typ)
}

func sortedPropertyNames(props openapi3.Schemas) []string {
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
