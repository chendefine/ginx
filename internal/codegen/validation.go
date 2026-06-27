package codegen

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func buildBindingRules(required bool, schemaRef *openapi3.SchemaRef) string {
	var rules []string

	if required {
		rules = append(rules, "required")
	}

	if schemaRef == nil || schemaRef.Value == nil {
		return strings.Join(rules, ",")
	}
	schema := schemaRef.Value

	if schema.Format != "" {
		if r := formatToValidator(schema.Format); r != "" {
			rules = append(rules, r)
		}
	}

	if len(schema.Enum) > 0 {
		var vals []string
		for _, v := range schema.Enum {
			vals = append(vals, fmt.Sprintf("%v", v))
		}
		rules = append(rules, "oneof="+strings.Join(vals, " "))
	}

	// Numeric bounds. OpenAPI 3.0 couples minimum/maximum with a boolean
	// exclusiveMinimum/exclusiveMaximum modifier; OpenAPI 3.1 makes them
	// standalone numeric bounds. ExclusiveBound carries both forms.
	switch {
	case schema.Min != nil && schema.ExclusiveMin.IsTrue():
		rules = append(rules, fmt.Sprintf("gt=%v", *schema.Min))
	case schema.Min != nil:
		rules = append(rules, fmt.Sprintf("gte=%v", *schema.Min))
	case schema.ExclusiveMin.Value != nil:
		rules = append(rules, fmt.Sprintf("gt=%v", *schema.ExclusiveMin.Value))
	}
	switch {
	case schema.Max != nil && schema.ExclusiveMax.IsTrue():
		rules = append(rules, fmt.Sprintf("lt=%v", *schema.Max))
	case schema.Max != nil:
		rules = append(rules, fmt.Sprintf("lte=%v", *schema.Max))
	case schema.ExclusiveMax.Value != nil:
		rules = append(rules, fmt.Sprintf("lt=%v", *schema.ExclusiveMax.Value))
	}

	if schema.MinLength != 0 {
		rules = append(rules, fmt.Sprintf("min=%d", schema.MinLength))
	}
	if schema.MaxLength != nil {
		rules = append(rules, fmt.Sprintf("max=%d", *schema.MaxLength))
	}

	if schema.MinItems != 0 {
		rules = append(rules, fmt.Sprintf("min=%d", schema.MinItems))
	}
	if schema.MaxItems != nil {
		rules = append(rules, fmt.Sprintf("max=%d", *schema.MaxItems))
	}

	if schema.UniqueItems {
		rules = append(rules, "unique")
	}

	if schema.MinProps != 0 {
		rules = append(rules, fmt.Sprintf("min=%d", schema.MinProps))
	}
	if schema.MaxProps != nil {
		rules = append(rules, fmt.Sprintf("max=%d", *schema.MaxProps))
	}

	if ext := extensionBinding(schema.Extensions); ext != "" {
		rules = append(rules, ext)
	}

	return strings.Join(rules, ",")
}

func formatToValidator(format string) string {
	switch format {
	case "email":
		return "email"
	case "uri":
		return "url"
	case "uuid":
		return "uuid"
	case "ipv4":
		return "ipv4"
	case "ipv6":
		return "ipv6"
	case "hostname":
		return "hostname"
	default:
		return ""
	}
}

func extensionBinding(extensions map[string]any) string {
	v, ok := extensions["x-binding"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
