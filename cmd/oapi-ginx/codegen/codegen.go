package codegen

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/tools/imports"
)

func Generate(cfg Config) ([]byte, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(cfg.SpecPath)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}

	if err := spec.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("validate spec: %w", err)
	}

	importsMap := make(map[string]bool)
	seen := make(map[string]bool)

	var allTypes []TypeDef

	if spec.Components != nil && spec.Components.Schemas != nil {
		schemaNames := sortedSchemaNames(spec.Components.Schemas)
		for _, name := range schemaNames {
			schemaRef := spec.Components.Schemas[name]
			typeName := ToCamelCase(name)
			types := ResolveSchema(typeName, schemaRef, importsMap, seen)
			allTypes = append(allTypes, types...)
		}
	}

	ops, extraTypes := ExtractOperations(spec, &cfg, importsMap, seen)
	allTypes = append(allTypes, extraTypes...)

	for _, op := range ops {
		if op.Request != nil && (len(op.Request.Fields) > 0 || len(op.Request.Embeds) > 0) {
			allTypes = append(allTypes, TypeDef{Struct: op.Request})
		}
		if op.Response != nil {
			allTypes = append(allTypes, *op.Response)
		}
	}

	// Apply custom type mappings
	if len(cfg.TypeMapping) > 0 {
		for i := range allTypes {
			applyTypeMapping(&allTypes[i], cfg.TypeMapping, importsMap)
		}
	}

	importList := sortedImports(importsMap)

	pkgName := cfg.PackageName
	if pkgName == "" {
		pkgName = "api"
	}

	data := &templateData{
		PackageName: pkgName,
		Imports:     importList,
		Types:       allTypes,
	}

	code, err := executeTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}

	formatted, err := imports.Process("generated.go", []byte(code), &imports.Options{
		Comments:  true,
		TabIndent: true,
		TabWidth:  8,
	})
	if err != nil {
		return []byte(code), nil
	}

	return formatted, nil
}

func applyTypeMapping(td *TypeDef, mapping map[string]string, imports map[string]bool) {
	if td.Struct != nil {
		for i, f := range td.Struct.Fields {
			if replacement, ok := mapping[f.Type]; ok {
				td.Struct.Fields[i].Type = replacement
				addImportForType(replacement, imports)
			}
			trimmed := f.Type
			if len(trimmed) > 0 && trimmed[0] == '*' {
				trimmed = trimmed[1:]
			}
			if replacement, ok := mapping[trimmed]; ok {
				if f.Type[0] == '*' {
					td.Struct.Fields[i].Type = "*" + replacement
				} else {
					td.Struct.Fields[i].Type = replacement
				}
				addImportForType(replacement, imports)
			}
		}
	}
	if td.Alias != nil {
		if replacement, ok := mapping[td.Alias.TargetType]; ok {
			td.Alias.TargetType = replacement
			addImportForType(replacement, imports)
		}
	}
}

func addImportForType(goType string, imports map[string]bool) {
	if idx := lastDotIndex(goType); idx > 0 {
		pkg := goType[:idx]
		if pkg[0] == '*' {
			pkg = pkg[1:]
		}
		imports[pkg] = true
	}
}

func lastDotIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func sortedSchemaNames(schemas openapi3.Schemas) []string {
	names := make([]string, 0, len(schemas))
	for k := range schemas {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func sortedImports(m map[string]bool) []string {
	var list []string
	for k := range m {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}
