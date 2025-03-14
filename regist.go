package ginx

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
)

var (
	markExist = struct{}{}

	waitParseStruct = map[reflect.Type]struct{}{}

	layouts  = []*structLayout{}
	services = []*serviceDefinition{}

	layoutExist  = map[string]*structLayout{}
	serviceExist = map[serviceEndpoint]*serviceDefinition{}

	tagsParse = []string{"json", "form", "uri"}
	pkgExist  = map[string]struct{}{}

	pbData string

	goTypeToPbType = map[reflect.Kind]string{
		reflect.Int: "int32", reflect.Uint: "uint32", // TODO: int是否应该转成int64待定
		reflect.Float32: "float", reflect.Float64: "double",

		reflect.Bool: "bool", reflect.String: "string",
		reflect.Int8: "int8", reflect.Int16: "int16", reflect.Int32: "int32", reflect.Int64: "int64",
		reflect.Uint8: "uint8", reflect.Uint16: "uint16", reflect.Uint32: "uint32", reflect.Uint64: "uint64",
	}

	nullType   = reflect.TypeOf(struct{}{})
	nullLayout = &structLayout{Type: nullType, Name: "Null"}
	anyLayout  = &structLayout{Type: nullType, Name: "Any"}

	reflectTypeOfCode = reflect.TypeOf(int32(0))
	reflectTypeOfMsg  = reflect.TypeOf("")

	reflectKindOfAny = reflect.TypeOf(Any{}).Kind()
)

type fieldMeta struct {
	Type    reflect.Type
	Name    string
	Form    string
	TagName string
}

type structLayout struct {
	Type   reflect.Type
	Name   string
	Fields []*fieldMeta
}

type serviceEndpoint struct {
	Method string
	Path   string
}

type serviceDefinition struct {
	serviceEndpoint
	FuncName  string
	ReqLayout *structLayout
	RspLayout *structLayout
}

func trimName(name string) string {
	if namePkgPrefix {
		return strings.ReplaceAll(name, ".", "_")
	} else {
		nameSplit := strings.Split(name, ".")
		return nameSplit[len(nameSplit)-1]
	}
}

func (layout structLayout) TrimName() string {
	return trimName(layout.Name)
}

func (layout structLayout) TrimNameWrap() string {
	return trimName(layout.Name)
}

func (service serviceDefinition) TrimFuncName() string {
	return trimName(service.FuncName)
}

func (layout structLayout) PbMessageString() string {
	var b strings.Builder
	b.WriteString("message ")
	b.WriteString(strcase.ToCamel(layout.TrimName()))
	b.WriteString(" {\n")

	for i, field := range layout.Fields {
		b.WriteString("  ")
		b.WriteString(field.Form)
		b.WriteString(" ")
		b.WriteString(field.TagName)
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(";\n")
	}

	b.WriteString("}\n\n")
	return b.String()
}

func (service serviceDefinition) PbMessageString() string {
	var b strings.Builder

	b.WriteString("  rpc ")
	b.WriteString(strcase.ToCamel(service.TrimFuncName()))
	b.WriteByte('(')
	b.WriteString(strcase.ToCamel(service.ReqLayout.TrimName()))
	b.WriteString(") returns (")
	b.WriteString(strcase.ToCamel(service.RspLayout.TrimName()))
	b.WriteString(") {\n")
	b.WriteString("    option (google.api.http) = {\n")
	b.WriteString("      ")
	b.WriteString(strings.ToLower(service.Method))
	b.WriteString(`: "`)
	b.WriteString(service.Path) // TODO: 转换URI路径参数, 即 /:param/ -> /{param}/
	b.WriteString("\"\n")
	if service.Method != http.MethodGet {
		b.WriteString("      body: \"*\"\n")
	}
	b.WriteString("    };\n  }\n")
	return b.String()
}

func parseField(field reflect.StructField) []*fieldMeta {
	if !field.IsExported() { // 不可导出, 则代表无需解析去pb字段
		return nil

	} else if field.Anonymous { // 匿名字段代表json不作嵌套, 因此解析该字段的数据结构体里的各个字段返回
		typ := field.Type
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return nil
		}
		layout := parseStruct(typ)
		layoutExist[layout.Name] = layout
		return layout.Fields
	}
	// 非匿名字段则只返回这个字段
	meta := fieldMeta{Type: field.Type, Name: field.Name, TagName: field.Name, Form: parseFieldForm(field.Type)}
	for _, tag := range tagsParse {
		if tagData, ok := field.Tag.Lookup(tag); !ok {
			continue
		} else if tagData == "-" {
			return nil
		} else {
			meta.TagName = strings.Split(tagData, ",")[0]
			break
		}
	}
	return []*fieldMeta{&meta}
}

func parseFieldForm(typ reflect.Type) string {
	switch kind := typ.Kind(); kind {
	case reflect.Pointer:
		return parseFieldForm(typ.Elem())
	case reflect.Array, reflect.Slice:
		elemName := parseFieldForm(typ.Elem())
		if strings.HasPrefix(elemName, "map<") {
			elemName = "google.protobuf.Any"
		} else if strings.HasPrefix(elemName, "repeated ") {
			panic(fmt.Errorf("repeated字段不能直接嵌套repeated: %s", typ.Name()))
		}
		return fmt.Sprintf("repeated %s", elemName)
	case reflect.Map:
		elemName := parseFieldForm(typ.Elem())
		if strings.HasPrefix(elemName, "map<") {
			elemName = "google.protobuf.Any"
		} else if strings.HasPrefix(elemName, "repeated ") {
			panic(fmt.Errorf("map字段不能直接嵌套repeated: %s", typ.Name()))
		}
		return fmt.Sprintf("map<%s,%s>", parseFieldForm(typ.Key()), elemName)
	case reflect.Interface, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return "google.protobuf.Any"
	case reflect.Struct:
		if _, ok := waitParseStruct[typ]; ok { // 当整个解析链路里有同一个struct待解析时, 就不再调parseStruct, 不然会死循环
			return strcase.ToCamel(trimName(typ.String()))

		} else {
			layout := parseStruct(typ)
			registStruct(layout)
			return strcase.ToCamel(layout.TrimName())
		}
	default:
		return goTypeToPbType[kind]
	}
}

func parseStruct(typ reflect.Type) *structLayout {
	if layout, ok := layoutExist[typ.String()]; ok {
		return layout
	}

	if typ.Kind() == reflectKindOfAny {
		return anyLayout
	} else if typ.Kind() != reflect.Struct {
		panic(fmt.Errorf("not a struct(%s)", typ.String()))
	}

	numField := typ.NumField()
	if numField == 0 {
		return nullLayout
	}

	waitParseStruct[typ] = struct{}{}
	defer delete(waitParseStruct, typ)

	pkgStructSplit := strings.Split(typ.PkgPath(), "/")
	pkgExist[pkgStructSplit[len(pkgStructSplit)-1]] = markExist

	layout := &structLayout{Type: typ, Name: typ.String(), Fields: make([]*fieldMeta, 0, numField)}
	for i := 0; i < numField; i++ {
		if fields := parseField(typ.Field(i)); fields != nil {
			layout.Fields = append(layout.Fields, fields...)
		}
	}
	return layout
}

func wrapRspStruct(layout *structLayout) *structLayout {
	wrapLayout := &structLayout{
		Name: layout.Name + "Wrap",
		Fields: []*fieldMeta{
			{Type: reflectTypeOfCode, Name: "Code", TagName: "code", Form: "int32"},
			{Type: reflectTypeOfMsg, Name: "Msg", TagName: "msg", Form: "string"},
			{Type: layout.Type, Name: "Data", TagName: "data", Form: parseFieldForm(layout.Type)},
		},
	}
	return wrapLayout
}

func registStruct(layout *structLayout) {
	if _, ok := layoutExist[layout.Name]; ok {
		return
	}
	layouts = append(layouts, layout)
	layoutExist[layout.Name] = layout
}

func registService(method, path string, funcName string, req, rsp *structLayout) {
	endpoint := serviceEndpoint{Method: method, Path: path}
	if _, ok := serviceExist[endpoint]; ok {
		return
	}

	service := &serviceDefinition{serviceEndpoint: endpoint, FuncName: funcName, ReqLayout: req, RspLayout: rsp}
	services = append(services, service)
	serviceExist[endpoint] = service
}

func regist(method string, router iRouter, path string, handle, req, rsp any, wrapRsp bool) {
	funcName := getFuncName(handle)
	reqLayout, rspLayout := parseStruct(reflect.TypeOf(req)), parseStruct(reflect.TypeOf(rsp))
	if wrapRsp {
		rspLayout = wrapRspStruct(rspLayout)
	}
	registStruct(reqLayout)
	registStruct(rspLayout)
	registService(method, joinPaths(router.BasePath(), path), funcName, reqLayout, rspLayout)
}
