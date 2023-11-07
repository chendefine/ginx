package ginx

import (
	"reflect"
	"strings"
	"sync"
)

// bindingPlan 由 register 时一次反射扫描得出, hot path 按 plan 选择性调用绑定器,
// 避免对空 tag 的结构体做无意义的反射遍历.
type bindingPlan struct {
	hasHeader    bool              // 存在 `header:"..."`
	hasURI       bool              // 存在 `uri:"..."`
	hasForm      bool              // 存在 `form:"..."`, 用于 query/form-post/multipart
	hasJSON      bool              // 存在 `json:"..."` 有效 name 或含嵌套结构
	hasDefaults  bool              // 存在 `default:"..."`, 决定是否调用 defaults.Set
	isEmpty      bool              // Req 结构体零字段, 完全跳过绑定
	fieldNameMap map[string]string // Go 字段名 -> tag 名, 用于校验错误提示, 优先级: json > form > uri > header
}

var planCache sync.Map // reflect.Type -> *bindingPlan

// buildBindingPlan 递归扫描 struct 字段(忽略匿名/嵌入不变), 返回 plan.
// Req 一般都很小, 这里为了简洁不做深度限制.
func buildBindingPlan(t reflect.Type) *bindingPlan {
	if t == nil {
		return &bindingPlan{isEmpty: true}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return &bindingPlan{isEmpty: true}
	}
	if cached, ok := planCache.Load(t); ok {
		return cached.(*bindingPlan)
	}
	plan := &bindingPlan{fieldNameMap: make(map[string]string)}
	scanType(t, plan, map[reflect.Type]struct{}{})
	if t.NumField() == 0 {
		plan.isEmpty = true
	}
	planCache.Store(t, plan)
	return plan
}

func scanType(t reflect.Type, plan *bindingPlan, seen map[reflect.Type]struct{}) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	if _, ok := seen[t]; ok {
		return
	}
	seen[t] = struct{}{}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		// fieldNameMap: 优先级 json > form > uri > header
		mapped := false
		if v, ok := f.Tag.Lookup("json"); ok {
			name := v
			if idx := strings.Index(v, ","); idx >= 0 {
				name = v[:idx]
			}
			if name != "" && name != "-" {
				plan.hasJSON = true
				plan.fieldNameMap[f.Name] = name
				mapped = true
			}
		}
		if v, ok := f.Tag.Lookup("form"); ok {
			name := v
			if idx := strings.Index(v, ","); idx >= 0 {
				name = v[:idx]
			}
			if name != "" && name != "-" {
				plan.hasForm = true
				if !mapped {
					plan.fieldNameMap[f.Name] = name
					mapped = true
				}
			}
		}
		if v, ok := f.Tag.Lookup("uri"); ok {
			plan.hasURI = true
			if !mapped && v != "" {
				plan.fieldNameMap[f.Name] = v
				mapped = true
			}
		}
		if v, ok := f.Tag.Lookup("header"); ok {
			plan.hasHeader = true
			if !mapped && v != "" {
				plan.fieldNameMap[f.Name] = v
			}
		}
		if _, ok := f.Tag.Lookup("default"); ok {
			plan.hasDefaults = true
		}

		// 嵌套结构体(命名或匿名)继续扫描
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			scanType(ft, plan, seen)
		}
	}
}
