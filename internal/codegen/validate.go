package codegen

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func validateComponentTypeNames(spec *openapi3.T) error {
	if spec == nil || spec.Components == nil || spec.Components.Schemas == nil {
		return nil
	}
	seen := make(map[string]string)
	for _, name := range sortedSchemaNames(spec.Components.Schemas) {
		goName := ToIdentifier(name)
		if prev, ok := seen[goName]; ok && prev != name {
			return fmt.Errorf("type name conflict: schemas %q and %q both generate %s; rename one schema or add a type_mapping/type_mapping_ext override", prev, name, goName)
		}
		seen[goName] = name
	}
	return nil
}

func validateOperationNames(ops []OperationDef) error {
	seenOps := make(map[string]string)
	seenTypes := make(map[string]string)
	for _, op := range ops {
		source := op.Method + " " + op.Path
		if prev, ok := seenOps[op.Name]; ok && prev != source {
			return fmt.Errorf("operation name conflict: %s and %s both generate %s; set unique operationId values", prev, source, op.Name)
		}
		seenOps[op.Name] = source

		for _, name := range []string{op.Name + "Req", op.Name + "Rsp"} {
			if name == op.Name+"Rsp" && (op.IsSSE || op.RspTypeName != name) {
				continue
			}
			if prev, ok := seenTypes[name]; ok && prev != source {
				return fmt.Errorf("generated type conflict: %s and %s both generate %s; set unique operationId values or rename schemas", prev, source, name)
			}
			seenTypes[name] = source
		}
	}
	return nil
}

func validateClientOperations(ops []OperationDef) error {
	for _, op := range ops {
		if op.IsSSE {
			continue
		}
		if hasMultipartFileFields(op) {
			return fmt.Errorf("client generation does not support multipart file upload for operation %s (%s %s); generate server/types only or model a dedicated client upload API", op.Name, op.Method, op.Path)
		}
	}
	return nil
}

func validateTypeDefs(types []TypeDef) error {
	seenTypes := make(map[string]string)
	for _, td := range types {
		name, kind := typeDefName(td)
		if name != "" {
			if prev, ok := seenTypes[name]; ok {
				return fmt.Errorf("generated type conflict: %s and %s both generate %s; rename schemas or adjust operationId values", prev, kind, name)
			}
			seenTypes[name] = kind
		}
		if td.Struct != nil {
			if err := validateStructDef(td.Struct); err != nil {
				return err
			}
		}
		if td.Enum != nil {
			if err := validateEnumDef(td.Enum); err != nil {
				return err
			}
		}
	}
	return nil
}

func typeDefName(td TypeDef) (string, string) {
	switch {
	case td.Struct != nil:
		return td.Struct.Name, "struct"
	case td.Enum != nil:
		return td.Enum.TypeName, "enum"
	case td.Alias != nil:
		return td.Alias.Name, "alias"
	default:
		return "", ""
	}
}

func validateStructDef(st *StructDef) error {
	seen := make(map[string]string)
	for _, embed := range st.Embeds {
		if prev, ok := seen[embed]; ok {
			return fmt.Errorf("field name conflict in %s: %q and %q both generate %s; rename the OpenAPI fields or split the schema", st.Name, prev, embed, embed)
		}
		seen[embed] = embed
	}
	for _, field := range st.Fields {
		source := fieldSourceName(field)
		if prev, ok := seen[field.Name]; ok && prev != source {
			return fmt.Errorf("field name conflict in %s: %q and %q both generate %s; rename one OpenAPI field", st.Name, prev, source, field.Name)
		}
		seen[field.Name] = source
	}
	return nil
}

func validateEnumDef(enum *EnumDef) error {
	seen := make(map[string]string)
	for _, value := range enum.Values {
		if prev, ok := seen[value.Name]; ok && prev != value.Value {
			return fmt.Errorf("enum const conflict in %s: values %s and %s both generate %s; rename one enum value", enum.TypeName, prev, value.Value, value.Name)
		}
		seen[value.Name] = value.Value
	}
	return nil
}

func fieldSourceName(field FieldDef) string {
	for _, key := range []string{"json", "form", "uri", "header"} {
		for _, tag := range field.Tags {
			if tag.Key == key && tag.Value != "" {
				return tag.Value
			}
		}
	}
	if field.Comment != "" {
		return field.Comment
	}
	return strings.TrimSpace(field.Name)
}
