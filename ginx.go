package ginx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/creasty/defaults"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	validator "github.com/go-playground/validator/v10"
)

var errResponseHandled = errors.New("ginx: response handled")

// HandlerFunc RPC 风格 handler 签名.
type HandlerFunc[Req, Rsp any] func(ctx context.Context, req *Req) (*Rsp, error)

// EmptyHandler 占位空 handler, 常用于 /healthz 等无请求无响应场景.
var EmptyHandler HandlerFunc[struct{}, struct{}] = func(context.Context, *struct{}) (*struct{}, error) {
	return nil, nil
}

// Any 是 map[string]any 的便捷别名.
type AnyMap = map[string]any

// successBody 成功响应在 dataWrap=true 时使用的标准包装体.
type successBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func defaultSuccessHandler(ctx context.Context, data any) (int, any) {
	return http.StatusOK, successBody{Code: 0, Msg: "", Data: data}
}

func defaultJSONRenderer(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// --- Public registration API ---

// GET 注册 GET 路由.
func GET[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodGet, path, fn, opts...)
}

// POST 注册 POST 路由.
func POST[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodPost, path, fn, opts...)
}

// PUT 注册 PUT 路由.
func PUT[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodPut, path, fn, opts...)
}

// PATCH 注册 PATCH 路由.
func PATCH[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodPatch, path, fn, opts...)
}

// DELETE 注册 DELETE 路由.
func DELETE[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodDelete, path, fn, opts...)
}

// HEAD 注册 HEAD 路由.
func HEAD[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodHead, path, fn, opts...)
}

// OPTIONS 注册 OPTIONS 路由.
func OPTIONS[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	register(r, http.MethodOptions, path, fn, opts...)
}

// Any 对 7 个常见 HTTP 方法都注册同一个 handler.
func Any[Req, Rsp any](r gin.IRoutes, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	for _, m := range []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions,
	} {
		register(r, m, path, fn, opts...)
	}
}

// Handle 在指定若干 HTTP method 上注册同一个 handler.
func Handle[Req, Rsp any](r gin.IRoutes, methods []string, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	for _, m := range methods {
		register(r, m, path, fn, opts...)
	}
}

// --- Internals ---

func register[Req, Rsp any](r gin.IRoutes, method, path string, fn HandlerFunc[Req, Rsp], opts ...RouteOption) {
	router, engine := engineOf(r)
	cfg := engine.resolveRoute(opts)

	var reqZero Req
	reqType := reflect.TypeOf(reqZero)
	plan := buildBindingPlan(reqType)

	handler := makeHandler(cfg, plan, fn)
	router.Handle(method, path, handler)

	if len(engine.onRegister) > 0 {
		info := RegisterInfo{
			Method:  method,
			Path:    path,
			ReqType: reqType,
			RspType: reflect.TypeOf((*Rsp)(nil)).Elem(),
		}
		for _, h := range engine.onRegister {
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
			if err := binding.Validator.ValidateStruct(&req); err != nil {
				writeBindingError(gc, cfg, plan, err)
				return
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

func sanitizeValidationError(err error, fieldNameMap map[string]string) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		return err.Error()
	}

	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		field := fe.Field()
		// 优先使用 tag 名
		if tagName, ok := fieldNameMap[field]; ok && tagName != "" {
			field = tagName
		}
		var msg string
		p := fe.Param()
		switch fe.Tag() {
		// required 系列
		case "required", "required_if", "required_unless",
			"required_with", "required_with_all",
			"required_without", "required_without_all":
			msg = field + " is required"
		// 排他系列
		case "excluded_if", "excluded_unless",
			"excluded_with", "excluded_with_all",
			"excluded_without", "excluded_without_all":
			msg = field + " must not be set"
		// 比较
		case "eq", "eq_ignore_case":
			msg = field + " must equal " + p
		case "ne", "ne_ignore_case":
			msg = field + " must not equal " + p
		case "gt":
			msg = field + " must be greater than " + p
		case "gte":
			msg = field + " must be greater than or equal to " + p
		case "lt":
			msg = field + " must be less than " + p
		case "lte":
			msg = field + " must be less than or equal to " + p
		case "min":
			msg = field + " must be at least " + p
		case "max":
			msg = field + " must be at most " + p
		case "len":
			msg = field + " length must be exactly " + p
		case "oneof":
			msg = field + " must be one of " + p
		case "unique":
			msg = field + " must contain unique values"
		// 字符串内容
		case "contains":
			msg = field + " must contain " + p
		case "containsany":
			msg = field + " must contain at least one of " + p
		case "containsrune":
			msg = field + " must contain character " + p
		case "excludes":
			msg = field + " must not contain " + p
		case "excludesall":
			msg = field + " must not contain any of " + p
		case "startswith":
			msg = field + " must start with " + p
		case "endswith":
			msg = field + " must end with " + p
		case "startsnotwith":
			msg = field + " must not start with " + p
		case "endsnotwith":
			msg = field + " must not end with " + p
		case "lowercase":
			msg = field + " must be lowercase"
		case "uppercase":
			msg = field + " must be uppercase"
		case "alpha":
			msg = field + " must contain only letters"
		case "alphanum":
			msg = field + " must contain only letters and numbers"
		case "alphanumunicode", "alphaunicode":
			msg = field + " must contain only unicode letters and numbers"
		case "ascii", "printascii":
			msg = field + " must contain only ASCII characters"
		case "multibyte":
			msg = field + " must contain multibyte characters"
		case "number", "numeric":
			msg = field + " must be a numeric value"
		case "boolean":
			msg = field + " must be a boolean value"
		case "json":
			msg = field + " must be valid JSON"
		// 格式
		case "email":
			msg = field + " must be a valid email"
		case "url", "url_encoded":
			msg = field + " must be a valid URL"
		case "uri":
			msg = field + " must be a valid URI"
		case "http_url":
			msg = field + " must be a valid HTTP URL"
		case "uuid", "uuid3", "uuid4", "uuid5",
			"uuid_rfc4122", "uuid3_rfc4122", "uuid4_rfc4122", "uuid5_rfc4122":
			msg = field + " must be a valid UUID"
		case "ulid":
			msg = field + " must be a valid ULID"
		case "ip", "ip_addr":
			msg = field + " must be a valid IP address"
		case "ipv4", "ip4_addr":
			msg = field + " must be a valid IPv4 address"
		case "ipv6", "ip6_addr":
			msg = field + " must be a valid IPv6 address"
		case "cidr", "cidrv4", "cidrv6":
			msg = field + " must be a valid CIDR notation"
		case "mac":
			msg = field + " must be a valid MAC address"
		case "hostname", "hostname_rfc1123", "fqdn":
			msg = field + " must be a valid hostname"
		case "hostname_port":
			msg = field + " must be a valid host:port"
		case "base64", "base64url", "base64rawurl":
			msg = field + " must be a valid base64 string"
		case "datetime":
			msg = field + " must match datetime format " + p
		case "timezone":
			msg = field + " must be a valid timezone"
		case "latitude":
			msg = field + " must be a valid latitude"
		case "longitude":
			msg = field + " must be a valid longitude"
		case "hexcolor", "rgb", "rgba", "hsl", "hsla", "iscolor", "html_encoded":
			msg = field + " must be a valid color"
		case "credit_card":
			msg = field + " must be a valid credit card number"
		case "isbn", "isbn10", "isbn13":
			msg = field + " must be a valid ISBN"
		case "issn":
			msg = field + " must be a valid ISSN"
		case "e164":
			msg = field + " must be a valid phone number (E.164)"
		case "ssn":
			msg = field + " must be a valid SSN"
		case "btc_addr", "btc_addr_bech32":
			msg = field + " must be a valid Bitcoin address"
		case "eth_addr":
			msg = field + " must be a valid Ethereum address"
		case "md5":
			msg = field + " must be a valid MD5 hash"
		case "sha256":
			msg = field + " must be a valid SHA256 hash"
		case "dir", "dirpath":
			msg = field + " must be a valid directory path"
		case "file", "filepath":
			msg = field + " must be a valid file path"
		case "cron":
			msg = field + " must be a valid cron expression"
		default:
			msg = field + " is invalid"
		}
		msgs = append(msgs, msg)
	}
	return strings.Join(msgs, "; ")
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
