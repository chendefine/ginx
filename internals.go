package ginx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/creasty/defaults"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	validator "github.com/go-playground/validator/v10"
)

// 预留: Engine 级默认 error http 状态码.
const (
	defaultErrHttpStatus = http.StatusInternalServerError
	defaultBadReqStatus  = http.StatusBadRequest
)

func register[Req, Rsp any](r gin.IRoutes, method, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	router, engine := engineOf(r)
	cfg := engine.resolveRoute(opts)

	var reqZero Req
	reqType := reflect.TypeOf(reqZero)
	plan := buildBindingPlan(reqType)

	handler := makeHandler(cfg, plan, fn)
	router.Handle(method, path, handler)

	engine.mu.RLock()
	hooks := engine.onRegister
	engine.mu.RUnlock()
	if len(hooks) > 0 {
		info := RegisterInfo{
			Method:  method,
			Path:    path,
			ReqType: reqType,
			RspType: reflect.TypeOf((*Rsp)(nil)).Elem(),
		}
		for _, h := range hooks {
			h(info)
		}
	}
}

func makeHandler[Req, Rsp any](cfg resolved, plan *bindingPlan, fn HandlerFunc[Req, Rsp]) gin.HandlerFunc {
	return func(gc *gin.Context) {
		var req Req

		if !plan.isEmpty {
			if plan.hasDefaults {
				_ = defaults.Set(&req)
			}
			if err := bindRequest(gc, cfg, plan, &req); err != nil {
				writeBindingError(gc, cfg, plan, err)
				return
			}
			if plan.hasBinding {
				if err := binding.Validator.ValidateStruct(&req); err != nil {
					writeBindingError(gc, cfg, plan, err)
					return
				}
			}
		}

		ctx := acquireContext(gc)
		defer releaseContext(ctx)

		rsp, err := invokeHandler(ctx, &req, cfg.interceptors, fn)

		if gc.IsAborted() {
			return
		}
		if err != nil {
			if errors.Is(err, errResponseHandled) {
				return
			}
			writeError(ctx, cfg, err)
			return
		}
		writeSuccess(ctx, cfg, rsp)
	}
}

// bindRequest 按 plan + Content-Type 选择性执行绑定, 只返回非校验错误;
// 校验错误由后续 ValidateStruct 统一处理(保证多源字段都校验到).
func bindRequest[Req any](gc *gin.Context, cfg resolved, plan *bindingPlan, req *Req) error {
	if plan.hasHeader {
		if err := gc.ShouldBindHeader(req); err != nil && !isValidationError(err) {
			return err
		}
	}
	if plan.hasCookie {
		if err := bindCookies(gc, req); err != nil && !isValidationError(err) {
			return err
		}
	}
	if plan.hasURI {
		if err := gc.ShouldBindUri(req); err != nil && !isValidationError(err) {
			return err
		}
	}
	if plan.hasForm {
		if err := gc.ShouldBindQuery(req); err != nil && !isValidationError(err) {
			return err
		}
	}
	ct := gc.ContentType()
	switch {
	case plan.hasJSON && isJSONContentType(ct):
		if err := bindJSON(gc, cfg, req); err != nil && !isValidationError(err) {
			return err
		}
	case plan.hasForm && isContentType(ct, "application/x-www-form-urlencoded"):
		if err := gc.ShouldBindWith(req, binding.FormPost); err != nil && !isValidationError(err) {
			return err
		}
	case plan.hasForm && isContentType(ct, "multipart/form-data"):
		if err := gc.ShouldBindWith(req, binding.FormMultipart); err != nil && !isValidationError(err) {
			return err
		}
	}
	return nil
}

func bindCookies(gc *gin.Context, obj any) error {
	values := make(map[string][]string)
	if gc != nil && gc.Request != nil {
		for _, cookie := range gc.Request.Cookies() {
			value, _ := url.QueryUnescape(cookie.Value)
			values[cookie.Name] = append(values[cookie.Name], value)
		}
	}
	return binding.MapFormWithTag(obj, values, "cookie")
}

func bindJSON[Req any](gc *gin.Context, cfg resolved, req *Req) error {
	if gc.Request == nil || gc.Request.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(gc.Request.Body)
	if cfg.jsonDecoderUseNumber {
		decoder.UseNumber()
	}
	if err := decoder.Decode(req); err != nil {
		if errors.Is(err, io.EOF) {
			// 空 body: 由后续 validator 处理 required 字段
			return nil
		}
		return err
	}
	return nil
}

func isContentType(have, want string) bool {
	if have == want {
		return true
	}
	if !strings.HasPrefix(have, want) {
		return false
	}
	rest := have[len(want):]
	return len(rest) > 0 && (rest[0] == ';' || rest[0] == ' ')
}

func isJSONContentType(have string) bool {
	if isContentType(have, "application/json") {
		return true
	}
	if semi := strings.IndexByte(have, ';'); semi >= 0 {
		have = have[:semi]
	}
	have = strings.TrimSpace(strings.ToLower(have))
	return strings.HasSuffix(have, "+json")
}

func isValidationError(err error) bool {
	var ve validator.ValidationErrors
	return errors.As(err, &ve)
}

type validationMessage struct {
	suffix    string
	withParam bool
}

var validationMessages = map[string]validationMessage{
	// required 系列
	"required":             {suffix: " is required"},
	"required_if":          {suffix: " is required"},
	"required_unless":      {suffix: " is required"},
	"required_with":        {suffix: " is required"},
	"required_with_all":    {suffix: " is required"},
	"required_without":     {suffix: " is required"},
	"required_without_all": {suffix: " is required"},

	// 排他系列
	"excluded_if":          {suffix: " must not be set"},
	"excluded_unless":      {suffix: " must not be set"},
	"excluded_with":        {suffix: " must not be set"},
	"excluded_with_all":    {suffix: " must not be set"},
	"excluded_without":     {suffix: " must not be set"},
	"excluded_without_all": {suffix: " must not be set"},

	// 比较
	"eq":             {suffix: " must equal ", withParam: true},
	"eq_ignore_case": {suffix: " must equal ", withParam: true},
	"ne":             {suffix: " must not equal ", withParam: true},
	"ne_ignore_case": {suffix: " must not equal ", withParam: true},
	"gt":             {suffix: " must be greater than ", withParam: true},
	"gte":            {suffix: " must be greater than or equal to ", withParam: true},
	"lt":             {suffix: " must be less than ", withParam: true},
	"lte":            {suffix: " must be less than or equal to ", withParam: true},
	"min":            {suffix: " must be at least ", withParam: true},
	"max":            {suffix: " must be at most ", withParam: true},
	"len":            {suffix: " length must be exactly ", withParam: true},
	"oneof":          {suffix: " must be one of ", withParam: true},
	"unique":         {suffix: " must contain unique values"},

	// 字符串内容
	"contains":        {suffix: " must contain ", withParam: true},
	"containsany":     {suffix: " must contain at least one of ", withParam: true},
	"containsrune":    {suffix: " must contain character ", withParam: true},
	"excludes":        {suffix: " must not contain ", withParam: true},
	"excludesall":     {suffix: " must not contain any of ", withParam: true},
	"startswith":      {suffix: " must start with ", withParam: true},
	"endswith":        {suffix: " must end with ", withParam: true},
	"startsnotwith":   {suffix: " must not start with ", withParam: true},
	"endsnotwith":     {suffix: " must not end with ", withParam: true},
	"lowercase":       {suffix: " must be lowercase"},
	"uppercase":       {suffix: " must be uppercase"},
	"alpha":           {suffix: " must contain only letters"},
	"alphanum":        {suffix: " must contain only letters and numbers"},
	"alphanumunicode": {suffix: " must contain only unicode letters and numbers"},
	"alphaunicode":    {suffix: " must contain only unicode letters and numbers"},
	"ascii":           {suffix: " must contain only ASCII characters"},
	"printascii":      {suffix: " must contain only ASCII characters"},
	"multibyte":       {suffix: " must contain multibyte characters"},
	"number":          {suffix: " must be a numeric value"},
	"numeric":         {suffix: " must be a numeric value"},
	"boolean":         {suffix: " must be a boolean value"},
	"json":            {suffix: " must be valid JSON"},

	// 格式
	"email":            {suffix: " must be a valid email"},
	"url":              {suffix: " must be a valid URL"},
	"url_encoded":      {suffix: " must be a valid URL"},
	"uri":              {suffix: " must be a valid URI"},
	"http_url":         {suffix: " must be a valid HTTP URL"},
	"uuid":             {suffix: " must be a valid UUID"},
	"uuid3":            {suffix: " must be a valid UUID"},
	"uuid4":            {suffix: " must be a valid UUID"},
	"uuid5":            {suffix: " must be a valid UUID"},
	"uuid_rfc4122":     {suffix: " must be a valid UUID"},
	"uuid3_rfc4122":    {suffix: " must be a valid UUID"},
	"uuid4_rfc4122":    {suffix: " must be a valid UUID"},
	"uuid5_rfc4122":    {suffix: " must be a valid UUID"},
	"ulid":             {suffix: " must be a valid ULID"},
	"ip":               {suffix: " must be a valid IP address"},
	"ip_addr":          {suffix: " must be a valid IP address"},
	"ipv4":             {suffix: " must be a valid IPv4 address"},
	"ip4_addr":         {suffix: " must be a valid IPv4 address"},
	"ipv6":             {suffix: " must be a valid IPv6 address"},
	"ip6_addr":         {suffix: " must be a valid IPv6 address"},
	"cidr":             {suffix: " must be a valid CIDR notation"},
	"cidrv4":           {suffix: " must be a valid CIDR notation"},
	"cidrv6":           {suffix: " must be a valid CIDR notation"},
	"mac":              {suffix: " must be a valid MAC address"},
	"hostname":         {suffix: " must be a valid hostname"},
	"hostname_rfc1123": {suffix: " must be a valid hostname"},
	"fqdn":             {suffix: " must be a valid hostname"},
	"hostname_port":    {suffix: " must be a valid host:port"},
	"base64":           {suffix: " must be a valid base64 string"},
	"base64url":        {suffix: " must be a valid base64 string"},
	"base64rawurl":     {suffix: " must be a valid base64 string"},
	"datetime":         {suffix: " must match datetime format ", withParam: true},
	"timezone":         {suffix: " must be a valid timezone"},
	"latitude":         {suffix: " must be a valid latitude"},
	"longitude":        {suffix: " must be a valid longitude"},
	"hexcolor":         {suffix: " must be a valid color"},
	"rgb":              {suffix: " must be a valid color"},
	"rgba":             {suffix: " must be a valid color"},
	"hsl":              {suffix: " must be a valid color"},
	"hsla":             {suffix: " must be a valid color"},
	"iscolor":          {suffix: " must be a valid color"},
	"html_encoded":     {suffix: " must be HTML encoded"},
	"credit_card":      {suffix: " must be a valid credit card number"},
	"isbn":             {suffix: " must be a valid ISBN"},
	"isbn10":           {suffix: " must be a valid ISBN"},
	"isbn13":           {suffix: " must be a valid ISBN"},
	"issn":             {suffix: " must be a valid ISSN"},
	"e164":             {suffix: " must be a valid phone number (E.164)"},
	"ssn":              {suffix: " must be a valid SSN"},
	"btc_addr":         {suffix: " must be a valid Bitcoin address"},
	"btc_addr_bech32":  {suffix: " must be a valid Bitcoin address"},
	"eth_addr":         {suffix: " must be a valid Ethereum address"},
	"md5":              {suffix: " must be a valid MD5 hash"},
	"sha256":           {suffix: " must be a valid SHA256 hash"},
	"dir":              {suffix: " must be a valid directory path"},
	"dirpath":          {suffix: " must be a valid directory path"},
	"file":             {suffix: " must be a valid file path"},
	"filepath":         {suffix: " must be a valid file path"},
	"cron":             {suffix: " must be a valid cron expression"},
}

func sanitizeValidationError(err error, fieldNameMap map[string]string) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		return err.Error()
	}

	if len(ve) == 1 {
		return formatFieldError(ve[0], fieldNameMap)
	}

	msgs := make([]string, len(ve))
	for i, fe := range ve {
		msgs[i] = formatFieldError(fe, fieldNameMap)
	}
	return strings.Join(msgs, "; ")
}

func formatFieldError(fe validator.FieldError, fieldNameMap map[string]string) string {
	field := fe.Field()
	if tagName, ok := fieldNameMap[field]; ok && tagName != "" {
		field = tagName
	}
	if message, ok := validationMessages[fe.Tag()]; ok {
		if message.withParam {
			return field + message.suffix + fe.Param()
		}
		return field + message.suffix
	}
	return field + " is invalid"
}

func invokeHandler[Req, Rsp any](ctx context.Context, req *Req, interceptors []Interceptor, fn HandlerFunc[Req, Rsp]) (*Rsp, error) {
	if len(interceptors) == 0 {
		return fn(ctx, req)
	}
	// 用单个闭包 + 索引递增代替每层各分配一个闭包, 将每次请求的堆分配从 O(N) 降为 O(1).
	idx := 0
	var call func() (any, error)
	call = func() (any, error) {
		if idx >= len(interceptors) {
			return fn(ctx, req)
		}
		ic := interceptors[idx]
		idx++
		return ic(ctx, req, call)
	}
	result, err := call()
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	rsp, ok := result.(*Rsp)
	if !ok {
		// 拦截器返回了错误类型, 属于编程错误, 直接 panic 以便在开发/测试阶段尽早暴露.
		panic(fmt.Sprintf("ginx: interceptor returned %T, want *%s", result, reflect.TypeOf((*Rsp)(nil)).Elem()))
	}
	return rsp, nil
}

func writeBindingError(gc *gin.Context, cfg resolved, plan *bindingPlan, err error) {
	status := defaultBadReqStatus
	if cfg.alwaysOK {
		status = http.StatusOK
	}
	if isValidationError(err) && cfg.validationHandler != nil {
		ctx := acquireContext(gc)
		defer releaseContext(ctx)
		if s, body := cfg.validationHandler(ctx, err); s > 0 {
			if cfg.alwaysOK {
				s = http.StatusOK
			}
			cfg.jsonRenderer(gc, s, body)
			gc.Abort()
			return
		}
	}
	msg := err.Error()
	if isValidationError(err) {
		msg = sanitizeValidationError(err, plan.fieldNameMap)
	}
	cfg.jsonRenderer(gc, status, successBody{Code: cfg.invalidArgCode, Msg: msg})
	gc.Abort()
}

func writeError(ctx context.Context, cfg resolved, err error) {
	gc, ok := GinContext(ctx)
	if !ok {
		return
	}
	status := defaultErrHttpStatus
	if cfg.alwaysOK {
		status = http.StatusOK
	}

	var ew *ErrWrap
	if errors.As(err, &ew) {
		if !cfg.alwaysOK && ew.HttpCode > 100 && ew.HttpCode < 600 {
			status = ew.HttpCode
		}
		cfg.jsonRenderer(gc, status, ew)
		gc.Abort()
		return
	}

	if cfg.errorHandler != nil {
		if s, body := cfg.errorHandler(ctx, err); s > 0 {
			if cfg.alwaysOK {
				s = http.StatusOK
			}
			cfg.jsonRenderer(gc, s, body)
			gc.Abort()
			return
		}
	}

	cfg.jsonRenderer(gc, status, successBody{Code: cfg.internalErrorCode, Msg: err.Error()})
	gc.Abort()
}

func writeSuccess(ctx context.Context, cfg resolved, rsp any) {
	gc, ok := GinContext(ctx)
	if !ok {
		return
	}
	if rsp != nil {
		if r, ok := rsp.(Response); ok {
			if err := r.WriteTo(gc); err != nil {
				_ = gc.Error(err)
			}
			return
		}
	}
	if cfg.dataWrap {
		status, body := cfg.successHandler(ctx, rsp)
		if status <= 0 {
			status = http.StatusOK
		}
		cfg.jsonRenderer(gc, status, body)
		return
	}
	cfg.jsonRenderer(gc, http.StatusOK, rsp)
}

// bindingPlan 由 register 时一次反射扫描得出, hot path 按 plan 选择性调用绑定器,
// 避免对空 tag 的结构体做无意义的反射遍历.
type bindingPlan struct {
	hasHeader    bool              // 存在 `header:"..."`
	hasCookie    bool              // 存在 `cookie:"..."`
	hasURI       bool              // 存在 `uri:"..."`
	hasForm      bool              // 存在 `form:"..."`, 用于 query/form-post/multipart
	hasJSON      bool              // 存在 `json:"..."` 有效 name 或含嵌套结构
	hasDefaults  bool              // 存在 `default:"..."`, 决定是否调用 defaults.Set
	hasBinding   bool              // 存在 `binding:"..."`, 决定是否调用 ValidateStruct
	isEmpty      bool              // Req 结构体零字段, 完全跳过绑定
	fieldNameMap map[string]string // Go 字段名 -> tag 名, 用于校验错误提示, 优先级: json > form > uri > header > cookie
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

		// fieldNameMap: 优先级 json > form > uri > header > cookie
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
				mapped = true
			}
		}
		if v, ok := f.Tag.Lookup("cookie"); ok {
			plan.hasCookie = true
			if !mapped && v != "" {
				plan.fieldNameMap[f.Name] = v
			}
		}
		if _, ok := f.Tag.Lookup("default"); ok {
			plan.hasDefaults = true
		}
		if _, ok := f.Tag.Lookup("binding"); ok {
			plan.hasBinding = true
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
