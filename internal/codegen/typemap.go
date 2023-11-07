package codegen

type typeMapping struct {
	GoType string
	Import string
}

var defaultTypeMap = map[string]map[string]typeMapping{
	"integer": {
		"":      {GoType: "int"},
		"int32": {GoType: "int32"},
		"int64": {GoType: "int64"},
	},
	"number": {
		"":       {GoType: "float64"},
		"float":  {GoType: "float32"},
		"double": {GoType: "float64"},
	},
	"string": {
		"":          {GoType: "string"},
		"date":      {GoType: "string"},
		"date-time": {GoType: "time.Time", Import: "time"},
		"byte":      {GoType: "[]byte"},
		"binary":    {GoType: "[]byte"},
		"uuid":      {GoType: "string"},
		"uri":       {GoType: "string"},
		"email":     {GoType: "string"},
		"hostname":  {GoType: "string"},
		"ipv4":      {GoType: "string"},
		"ipv6":      {GoType: "string"},
	},
	"boolean": {
		"": {GoType: "bool"},
	},
}

func MapType(oaType, format string) (goType string, imp string) {
	if formats, ok := defaultTypeMap[oaType]; ok {
		if m, ok := formats[format]; ok {
			return m.GoType, m.Import
		}
		if m, ok := formats[""]; ok {
			return m.GoType, m.Import
		}
	}
	return "any", ""
}
