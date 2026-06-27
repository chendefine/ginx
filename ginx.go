package ginx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
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

// Event 表示一个 SSE event.
type Event struct {
	ID    string
	Event string
	Data  any
	Retry uint
}

// Sender 负责向客户端推送 SSE 事件.
type Sender func(Event) error

// SSEHandler 是 SSE 场景的 RPC 风格签名.
type SSEHandler[Req any] func(ctx context.Context, req *Req, send Sender) error

// successBody 成功响应在 dataWrap=true 时使用的标准包装体.
type successBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
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

// SSE 注册 SSE 路由. SSE 响应天然不走 dataWrap.
func SSE[Req any](r gin.IRoutes, path string, fn SSEHandler[Req], opts ...RouteOption) {
	register(r, http.MethodGet, path, func(ctx context.Context, req *Req) (*struct{}, error) {
		SetHeader(ctx, "Content-Type", "text/event-stream")
		SetHeader(ctx, "Cache-Control", "no-cache")
		SetHeader(ctx, "Connection", "keep-alive")
		gc, ok := GinContext(ctx)
		if !ok {
			return nil, errors.New("ginx: context does not contain *gin.Context")
		}
		sender := newSSESender(gc)
		if err := fn(ctx, req, sender); err != nil {
			return nil, err
		}
		return nil, errResponseHandled
	}, append([]RouteOption{NoDataWrap()}, opts...)...)
}

func newSSESender(c *gin.Context) Sender {
	return func(evt Event) error {
		if err := sse.Encode(c.Writer, sse.Event{
			Id:    evt.ID,
			Event: evt.Event,
			Data:  evt.Data,
			Retry: evt.Retry,
		}); err != nil {
			return err
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
}

// JSONLinesSender writes one JSON value to the response stream. Each call
// produces a single compact JSON record followed by '\n' and an immediate
// flush, matching the NDJSON / JSON Lines wire format.
//
// The item type is untyped (any): OpenAPI 3.2's per-item itemSchema is not
// preserved by kin-openapi (the field is silently dropped at parse time), and
// OpenAPI 3.1 has no per-item type at all. Callers marshal whatever domain
// object they wish; generated handlers do not constrain the item.
type JSONLinesSender func(item any) error

// JSONLinesHandler 是 JSON Lines / NDJSON 流式场景的 RPC 风格签名.
type JSONLinesHandler[Req any] func(ctx context.Context, req *Req, send JSONLinesSender) error

// JSONLines 注册一条 JSON Lines / NDJSON 流式路由. 每个 item 经 send 写出为
// 紧凑 JSON + '\n' 并立即 flush, 响应 Content-Type 为 application/x-ndjson.
// 与 SSE 一样, JSON Lines 响应不走 dataWrap.
//
// method 显式作为参数 (不同于必须 GET 的 SSE): NDJSON 惯例是 POST, 但并不强制.
func JSONLines[Req any](r gin.IRoutes, method, path string, fn JSONLinesHandler[Req], opts ...RouteOption) {
	register(r, method, path, func(ctx context.Context, req *Req) (*struct{}, error) {
		SetHeader(ctx, "Content-Type", "application/x-ndjson")
		SetHeader(ctx, "Cache-Control", "no-cache")
		SetHeader(ctx, "Connection", "keep-alive")
		gc, ok := GinContext(ctx)
		if !ok {
			return nil, errors.New("ginx: context does not contain *gin.Context")
		}
		sender := newJSONLinesSender(gc)
		if err := fn(ctx, req, sender); err != nil {
			return nil, err
		}
		return nil, errResponseHandled
	}, append([]RouteOption{NoDataWrap()}, opts...)...)
}

func newJSONLinesSender(c *gin.Context) JSONLinesSender {
	// json.Encoder.Encode writes compact JSON and appends '\n' itself, so each
	// call yields exactly one NDJSON record. We still flush explicitly so records
	// reach the wire per-item rather than in gin's default buffer.
	enc := json.NewEncoder(c.Writer)
	return func(item any) error {
		if err := enc.Encode(item); err != nil {
			return err
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
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
