package ginx

import (
	"context"
	"net/http"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
)

// Engine 保存 ginx 实例级配置, 所有 package 级 GET/POST 等函数默认作用于 defaultEngine,
// 通过 engine.Wrap(router) 产生的 *Router 可以将某棵路由树绑定到特定 Engine.
type Engine struct {
	mu                   sync.RWMutex
	dataWrap             bool
	invalidArgCode       int
	internalErrorCode    int
	jsonDecoderUseNumber bool
	strictJSONBody       bool
	exposeInternalError  bool
	internalErrorMessage string

	errorHandler      ErrorHandler
	validationHandler ValidationErrorHandler
	successHandler    SuccessHandler
	jsonRenderer      JSONRenderer

	interceptors []Interceptor
	onRegister   []RegisterHook
}

// ErrorHandler 将业务 error 转换为 HTTP 状态码 + 响应体.
// 当 handler 返回 *ErrWrap 时, 默认走内置逻辑; 其它 error 会先经过这里.
// 返回 httpStatus<=0 表示沿用默认处理.
type ErrorHandler func(ctx context.Context, err error) (httpStatus int, body any)

// ValidationErrorHandler 专门处理 validator 校验错误, 以便脱敏/国际化.
// 返回 httpStatus<=0 表示沿用默认处理.
type ValidationErrorHandler func(ctx context.Context, err error) (httpStatus int, body any)

// SuccessHandler 把成功返回值转成 HTTP 状态码与响应体.
// 仅在 dataWrap=true 时生效; 返回 httpStatus<=0 时视为 200.
type SuccessHandler func(ctx context.Context, data any) (httpStatus int, body any)

// JSONRenderer 负责把 JSON 类型响应写到客户端.
type JSONRenderer func(c *gin.Context, status int, body any)

// Interceptor 在 handler 真正执行前后插入逻辑. next() 执行下一层.
// req/rsp 为类型擦除, 需要时可用 reflect 检查.
// 注意: next 在单次请求中只能调用一次, 重复调用会 panic.
type Interceptor func(ctx context.Context, req any, next func() (any, error)) (any, error)

// RegisterInfo 路由注册时的元信息, 供外部生成 OpenAPI 等.
type RegisterInfo struct {
	Method  string
	Path    string
	ReqType reflect.Type
	RspType reflect.Type
}

// RegisterHook 每次路由注册时触发.
type RegisterHook func(info RegisterInfo)

// EngineOption 函数式配置.
type EngineOption func(*Engine)

// New 创建 Engine 实例, 未指定的配置走内建默认值.
func New(opts ...EngineOption) *Engine {
	e := &Engine{
		dataWrap:             true,
		invalidArgCode:       1,
		internalErrorCode:    2,
		exposeInternalError:  true,
		internalErrorMessage: http.StatusText(http.StatusInternalServerError),
		successHandler:       defaultSuccessHandler,
		jsonRenderer:         defaultJSONRenderer,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithDataWrap 控制成功响应是否用 {code,msg,data} 包装; 默认 true.
func WithDataWrap(b bool) EngineOption {
	return func(e *Engine) { e.dataWrap = b }
}

// WithInvalidArgCode 覆盖参数校验失败时的业务 code; 默认 1.
func WithInvalidArgCode(code int) EngineOption {
	return func(e *Engine) { e.invalidArgCode = code }
}

// WithInternalErrorCode 覆盖非 *ErrWrap error 时的业务 code; 默认 2.
func WithInternalErrorCode(code int) EngineOption {
	return func(e *Engine) { e.internalErrorCode = code }
}

// WithErrorHandler 注册自定义 error -> (httpStatus, body) 转换器.
func WithErrorHandler(h ErrorHandler) EngineOption {
	return func(e *Engine) { e.errorHandler = h }
}

// WithValidationErrorHandler 专门处理 validator 产生的校验错误.
func WithValidationErrorHandler(h ValidationErrorHandler) EngineOption {
	return func(e *Engine) { e.validationHandler = h }
}

// WithSuccessHandler 自定义 dataWrap=true 时的成功响应包装.
func WithSuccessHandler(h SuccessHandler) EngineOption {
	return func(e *Engine) { e.successHandler = h }
}

// WithJSONRenderer 自定义 JSON 响应写出方式.
func WithJSONRenderer(r JSONRenderer) EngineOption {
	return func(e *Engine) { e.jsonRenderer = r }
}

// WithInterceptor 追加一个拦截器, 多次调用按注册顺序洋葱包裹(最先注册的在最外层).
func WithInterceptor(i Interceptor) EngineOption {
	return func(e *Engine) { e.interceptors = append(e.interceptors, i) }
}

// WithOnRegister 注册路由时触发, 可用于生成 OpenAPI.
func WithOnRegister(h RegisterHook) EngineOption {
	return func(e *Engine) { e.onRegister = append(e.onRegister, h) }
}

// WithJsonDecoderUseNumber 开启 json.Decoder.UseNumber(), 仅作用于当前 Engine.
func WithJsonDecoderUseNumber(b bool) EngineOption {
	return func(e *Engine) { e.jsonDecoderUseNumber = b }
}

// WithStrictJSONBody 开启严格 JSON body 解析. 开启后 Decode 成功后仍会检查是否存在
// trailing token; 存在额外 JSON 值时按绑定错误返回. 默认 false 以保持兼容.
func WithStrictJSONBody(b bool) EngineOption {
	return func(e *Engine) { e.strictJSONBody = b }
}

// WithExposeInternalError 控制普通 error 是否直接暴露 err.Error().
// 默认 true 以保持兼容; 公网服务建议设置为 false 并配合 WithInternalErrorMessage.
func WithExposeInternalError(b bool) EngineOption {
	return func(e *Engine) { e.exposeInternalError = b }
}

// WithInternalErrorMessage 设置普通 error 脱敏后的统一文案, 并关闭原始错误暴露.
// 空字符串会回退为 HTTP 500 的标准文案.
func WithInternalErrorMessage(msg string) EngineOption {
	return func(e *Engine) {
		e.exposeInternalError = false
		if msg == "" {
			msg = http.StatusText(http.StatusInternalServerError)
		}
		e.internalErrorMessage = msg
	}
}

// Router 绑定了 Engine 的路由组, 传给 GET/POST 等函数时会自动使用对应 Engine.
type Router struct {
	gin.IRoutes
	engine *Engine
}

// Wrap 把任意 gin.IRoutes 绑定到该 Engine, 返回的 *Router 可直接传给 ginx.GET/POST...
func (e *Engine) Wrap(r gin.IRoutes) *Router {
	return &Router{IRoutes: r, engine: e}
}

// Group 在 Engine 上创建一个分组, 等价于 Wrap(r.Group(prefix)).
// r 可以是 *gin.Engine 或 *gin.RouterGroup.
func (e *Engine) Group(r gin.IRouter, prefix string, handlers ...gin.HandlerFunc) *Router {
	return &Router{IRoutes: r.Group(prefix, handlers...), engine: e}
}

// defaultEngine 承载 package 级 GET/POST/Configure 等 API.
var defaultEngine = New()

// Default 返回 defaultEngine, 便于上层在需要时直接拿到.
func Default() *Engine { return defaultEngine }

// Configure 在当前 Engine 上应用若干 Option. 已注册路由持有注册时的配置快照,
// 因此本方法只影响后续注册的路由.
func (e *Engine) Configure(opts ...EngineOption) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, opt := range opts {
		opt(e)
	}
}

// Configure 便捷地在 defaultEngine 上应用若干 Option.
func Configure(opts ...EngineOption) {
	defaultEngine.Configure(opts...)
}

// SetDataWrap 便捷调用 Configure(WithDataWrap(b)).
func SetDataWrap(b bool) { Configure(WithDataWrap(b)) }

// SetInvalidArgumentCode 便捷调用 Configure(WithInvalidArgCode(c)).
func SetInvalidArgumentCode(c int) { Configure(WithInvalidArgCode(c)) }

// SetInternalServerErrorCode 便捷调用 Configure(WithInternalErrorCode(c)).
func SetInternalServerErrorCode(c int) { Configure(WithInternalErrorCode(c)) }

// SetJsonDecoderUseNumber 便捷调用 Configure(WithJsonDecoderUseNumber(b)).
func SetJsonDecoderUseNumber(b bool) { Configure(WithJsonDecoderUseNumber(b)) }

// SetStrictJSONBody 便捷调用 Configure(WithStrictJSONBody(b)).
func SetStrictJSONBody(b bool) { Configure(WithStrictJSONBody(b)) }

// SetExposeInternalError 便捷调用 Configure(WithExposeInternalError(b)).
func SetExposeInternalError(b bool) { Configure(WithExposeInternalError(b)) }

// SetInternalErrorMessage 便捷调用 Configure(WithInternalErrorMessage(msg)).
func SetInternalErrorMessage(msg string) { Configure(WithInternalErrorMessage(msg)) }

// --- Route level options ---

// RouteOption 单路由级配置, 覆盖 Engine 的默认值.
type RouteOption func(*routeConfig)

type routeConfig struct {
	dataWrap      *bool // nil 表示沿用 Engine
	alwaysOK      bool
	successStatus int
	interceptors  []Interceptor
}

// WrapData 强制该路由走 {code,msg,data} 包装.
func WrapData() RouteOption {
	return func(c *routeConfig) { b := true; c.dataWrap = &b }
}

// NoDataWrap 强制该路由不走 data 包装, 直接输出 handler 返回值.
func NoDataWrap() RouteOption {
	return func(c *routeConfig) { b := false; c.dataWrap = &b }
}

// AlwaysOK 不论 handler 是否出错, HTTP 状态始终 200.
func AlwaysOK() RouteOption {
	return func(c *routeConfig) { c.alwaysOK = true }
}

// SuccessStatus 为当前路由设置固定的 2xx 成功状态码.
// 非 2xx 值在路由注册时 panic，避免静默违反 HTTP/OpenAPI 契约.
func SuccessStatus(code int) RouteOption {
	return func(c *routeConfig) {
		if code < http.StatusOK || code >= http.StatusMultipleChoices {
			panic("ginx: success status must be between 200 and 299")
		}
		c.successStatus = code
	}
}

// RouteInterceptor 追加路由级拦截器, 位于 Engine 拦截器之后(更内层).
func RouteInterceptor(i Interceptor) RouteOption {
	return func(c *routeConfig) { c.interceptors = append(c.interceptors, i) }
}

func (e *Engine) resolveRoute(opts []RouteOption) resolved {
	rc := routeConfig{}
	for _, opt := range opts {
		opt(&rc)
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	r := resolved{
		dataWrap:             e.dataWrap,
		alwaysOK:             rc.alwaysOK,
		successStatus:        rc.successStatus,
		invalidArgCode:       e.invalidArgCode,
		internalErrorCode:    e.internalErrorCode,
		jsonDecoderUseNumber: e.jsonDecoderUseNumber,
		strictJSONBody:       e.strictJSONBody,
		exposeInternalError:  e.exposeInternalError,
		internalErrorMessage: e.internalErrorMessage,
		errorHandler:         e.errorHandler,
		validationHandler:    e.validationHandler,
		successHandler:       e.successHandler,
		jsonRenderer:         e.jsonRenderer,
	}
	if rc.dataWrap != nil {
		r.dataWrap = *rc.dataWrap
	}
	if n := len(e.interceptors) + len(rc.interceptors); n > 0 {
		r.interceptors = make([]Interceptor, 0, n)
		r.interceptors = append(r.interceptors, e.interceptors...)
		r.interceptors = append(r.interceptors, rc.interceptors...)
	}
	return r
}

// resolved 是实际执行时用到的不可变配置快照.
type resolved struct {
	dataWrap             bool
	alwaysOK             bool
	successStatus        int
	invalidArgCode       int
	internalErrorCode    int
	jsonDecoderUseNumber bool
	strictJSONBody       bool
	exposeInternalError  bool
	internalErrorMessage string
	errorHandler         ErrorHandler
	validationHandler    ValidationErrorHandler
	successHandler       SuccessHandler
	jsonRenderer         JSONRenderer
	interceptors         []Interceptor
}

// engineOf 从注册入参解析 Engine 与底层 gin.IRoutes.
func engineOf(r gin.IRoutes) (gin.IRoutes, *Engine) {
	if rr, ok := r.(*Router); ok {
		return rr.IRoutes, rr.engine
	}
	return r, defaultEngine
}
