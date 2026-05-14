package codegen

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/tools/imports"
)

type GenerateResult struct {
	Types  []byte
	Server []byte
	Client []byte
	Spec   []byte
}

func Generate(cfg Config) ([]byte, error) {
	result, err := GenerateMulti(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Output.IsMultiFile() {
		return result.Types, nil
	}

	return result.Types, nil
}

func GenerateMulti(cfg Config) (*GenerateResult, error) {
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

	if err := validateComponentTypeNames(spec); err != nil {
		return nil, err
	}

	if spec.Components != nil && spec.Components.Schemas != nil {
		schemaNames := sortedSchemaNames(spec.Components.Schemas)
		for _, name := range schemaNames {
			schemaRef := spec.Components.Schemas[name]
			typeName := ToCamelCase(name)
			types := ResolveSchema(typeName, schemaRef, importsMap, seen)
			allTypes = append(allTypes, types...)
		}
	}

	ops, extraTypes, err := ExtractOperations(spec, &cfg, importsMap, seen)
	if err != nil {
		return nil, err
	}
	allTypes = append(allTypes, extraTypes...)

	generateServer := cfg.ShouldGenerateServer()
	generateClient := cfg.ShouldGenerateClient()

	for _, op := range ops {
		if op.Request != nil {
			if generateServer || generateClient || len(op.Request.Fields) > 0 || len(op.Request.Embeds) > 0 {
				allTypes = append(allTypes, TypeDef{Struct: op.Request})
			}
		}
		if op.Response != nil {
			allTypes = append(allTypes, *op.Response)
		}
	}

	if generateServer && len(ops) > 0 {
		importsMap["context"] = true
		importsMap["github.com/chendefine/ginx"] = true
		importsMap["github.com/gin-gonic/gin"] = true
	}

	if len(cfg.TypeMapping) > 0 {
		for i := range allTypes {
			applyTypeMapping(&allTypes[i], cfg.TypeMapping, importsMap)
		}
	}
	if len(cfg.TypeMappingExt) > 0 {
		for i := range allTypes {
			applyTypeMappingExt(&allTypes[i], cfg.TypeMappingExt, importsMap)
		}
	}
	if err := validateOperationNames(ops); err != nil {
		return nil, err
	}
	if err := validateTypeDefs(allTypes); err != nil {
		return nil, err
	}

	pkgName := cfg.PackageName
	if pkgName == "" {
		pkgName = "api"
	}

	skipFmt := cfg.OutputOptions.SkipFmt
	result := &GenerateResult{}

	if cfg.Output.IsMultiFile() {
		typesImports := filterTypesImports(importsMap)
		typesCode, err := executeTypesTemplate(&typesTemplateData{
			PackageName:       pkgName,
			GenerateDirective: cfg.GenerateDirective,
			Imports:           sortedImports(typesImports),
			Types:             allTypes,
		})
		if err != nil {
			return nil, fmt.Errorf("render types: %w", err)
		}
		result.Types, err = formatCode(typesCode, skipFmt)
		if err != nil {
			return nil, err
		}

		if cfg.Output.Server != "" && generateServer && len(ops) > 0 {
			serverImports := map[string]bool{
				"context":                    true,
				"github.com/chendefine/ginx": true,
				"github.com/gin-gonic/gin":   true,
			}
			for _, op := range ops {
				if op.RspTypeName == "ginx.FileRsp" || op.RspTypeName == "ginx.StringRsp" || op.RspTypeName == "ginx.RedirectRsp" {
					serverImports["github.com/chendefine/ginx"] = true
				}
			}
			serverCode, err := executeServerTemplate(&serverTemplateData{
				PackageName:       pkgName,
				GenerateDirective: cfg.GenerateDirective,
				Imports:           sortedImports(serverImports),
				Operations:        ops,
				ServerName:        cfg.GetServerName(),
			})
			if err != nil {
				return nil, fmt.Errorf("render server: %w", err)
			}
			result.Server, err = formatCode(serverCode, skipFmt)
			if err != nil {
				return nil, err
			}
		}

		if cfg.Output.Spec != "" {
			specBase64, err := CompressSpec(cfg.SpecPath)
			if err != nil {
				return nil, err
			}
			specCode, err := executeSpecTemplate(&specTemplateData{
				PackageName:       pkgName,
				GenerateDirective: cfg.GenerateDirective,
				SpecBase64:        specBase64,
			})
			if err != nil {
				return nil, fmt.Errorf("render spec: %w", err)
			}
			result.Spec, err = formatCode(specCode, skipFmt)
			if err != nil {
				return nil, err
			}
		}

		if cfg.Output.Client != "" && generateClient && len(ops) > 0 {
			clientImports := map[string]bool{
				"context":                    true,
				"fmt":                        true,
				"github.com/chendefine/ginx": true,
				"resty.dev/v3":               true,
			}
			if hasClientCookieParameters(ops) {
				clientImports["net/http"] = true
			}
			if hasSSEOperations(ops) {
				clientImports["strings"] = true
				clientImports["net/url"] = true
			}
			clientCode, err := executeClientTemplate(&clientTemplateData{
				PackageName:       pkgName,
				GenerateDirective: cfg.GenerateDirective,
				Imports:           sortedImports(clientImports),
				Operations:        ops,
				ServerName:        cfg.GetServerName(),
			})
			if err != nil {
				return nil, fmt.Errorf("render client: %w", err)
			}
			result.Client, err = formatCode(clientCode, skipFmt)
			if err != nil {
				return nil, err
			}
		}
	} else {
		allImports := sortedImports(importsMap)
		if generateClient && len(ops) > 0 {
			importsMap["fmt"] = true
			importsMap["github.com/chendefine/ginx"] = true
			importsMap["resty.dev/v3"] = true
			if hasClientCookieParameters(ops) {
				importsMap["net/http"] = true
			}
			if hasSSEOperations(ops) {
				importsMap["strings"] = true
				importsMap["net/url"] = true
			}
			allImports = sortedImports(importsMap)
		}
		code, err := executeCombinedTemplate(&combinedTemplateData{
			PackageName:       pkgName,
			GenerateDirective: cfg.GenerateDirective,
			Imports:           allImports,
			Types:             allTypes,
			Operations:        ops,
			GenerateServer:    generateServer,
			GenerateClient:    generateClient,
			ServerName:        cfg.GetServerName(),
		})
		if err != nil {
			return nil, fmt.Errorf("render template: %w", err)
		}
		result.Types, err = formatCode(code, skipFmt)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func formatCode(code string, skipFmt bool) ([]byte, error) {
	if skipFmt {
		return []byte(code), nil
	}
	formatted, err := imports.Process("generated.go", []byte(code), &imports.Options{
		Comments:  true,
		TabIndent: true,
		TabWidth:  8,
	})
	if err != nil {
		return nil, fmt.Errorf("format generated code: %w", err)
	}
	return formatted, nil
}

func filterTypesImports(all map[string]bool) map[string]bool {
	serverOnly := map[string]bool{
		"context":                    true,
		"github.com/chendefine/ginx": true,
		"github.com/gin-gonic/gin":   true,
	}
	result := make(map[string]bool)
	for k := range all {
		if !serverOnly[k] {
			result[k] = true
		}
	}
	return result
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

func applyTypeMappingExt(td *TypeDef, mapping map[string]TypeMappingExt, imports map[string]bool) {
	rewrite := func(goType string) string {
		if m, ok := mapping[goType]; ok && m.Type != "" {
			if m.Import != "" {
				imports[m.Import] = true
			} else {
				addImportForType(m.Type, imports)
			}
			return m.Type
		}
		if len(goType) > 0 && goType[0] == '*' {
			if m, ok := mapping[goType[1:]]; ok && m.Type != "" {
				if m.Import != "" {
					imports[m.Import] = true
				} else {
					addImportForType(m.Type, imports)
				}
				return "*" + m.Type
			}
		}
		return goType
	}
	if td.Struct != nil {
		for i, f := range td.Struct.Fields {
			td.Struct.Fields[i].Type = rewrite(f.Type)
		}
	}
	if td.Alias != nil {
		td.Alias.TargetType = rewrite(td.Alias.TargetType)
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

func hasSSEOperations(ops []OperationDef) bool {
	for _, op := range ops {
		if op.IsSSE {
			return true
		}
	}
	return false
}

func hasClientCookieParameters(ops []OperationDef) bool {
	for _, op := range ops {
		if !op.IsSSE && skipForClient(op) {
			continue
		}
		if len(filterCookieParams(op.Request)) > 0 {
			return true
		}
	}
	return false
}
