package ginx

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ginContextKey struct{}

var internalGinContextKey = ginContextKey{}

// Context 封装 *gin.Context 并实现 context.Context 接口.
//
// Deprecated: 业务路径现在使用标准 context.WithValue 包装 *gin.Context, 避免请求
// 返回后复用对象导致派生 context 失效. 该类型保留用于兼容和测试.
type Context struct {
	parent context.Context
	gc     *gin.Context
}

func requestContext(gc *gin.Context) context.Context {
	if gc != nil && gc.Request != nil {
		return gc.Request.Context()
	}
	return context.Background()
}

func acquireContext(gc *gin.Context) context.Context {
	return context.WithValue(requestContext(gc), internalGinContextKey, gc)
}

func releaseContext(context.Context) {
}

// newContext 直接构造, 主要用于测试. 业务路径走 acquireContext.
func newContext(gc *gin.Context) *Context {
	return &Context{parent: requestContext(gc), gc: gc}
}

// Deadline 实现 context.Context.
func (c *Context) Deadline() (time.Time, bool) { return c.parent.Deadline() }

// Done 实现 context.Context.
func (c *Context) Done() <-chan struct{} { return c.parent.Done() }

// Err 实现 context.Context.
func (c *Context) Err() error { return c.parent.Err() }

// Value 实现 context.Context.
func (c *Context) Value(key any) any {
	if key == internalGinContextKey {
		return c.gc
	}
	return c.parent.Value(key)
}

// GinContext 返回底层 *gin.Context 作为显式逃逸出口.
func GinContext(ctx context.Context) (*gin.Context, bool) {
	if c, ok := ctx.(*Context); ok && c.gc != nil {
		return c.gc, true
	}
	gc, ok := ctx.Value(internalGinContextKey).(*gin.Context)
	if !ok || gc == nil {
		return nil, false
	}
	return gc, true
}

// Get 获取中间件设置的值.
func Get(ctx context.Context, key string) (any, bool) {
	gc, ok := GinContext(ctx)
	if !ok {
		return nil, false
	}
	return gc.Get(key)
}

// MustGet 不存在时 panic.
func MustGet(ctx context.Context, key string) any {
	gc, ok := GinContext(ctx)
	if !ok {
		panic("ginx: context does not contain *gin.Context")
	}
	return gc.MustGet(key)
}

// Set 设置键值对.
func Set(ctx context.Context, key string, value any) {
	if gc, ok := GinContext(ctx); ok {
		gc.Set(key, value)
	}
}

// GetHeader 获取请求头.
func GetHeader(ctx context.Context, key string) string {
	gc, ok := GinContext(ctx)
	if !ok {
		return ""
	}
	return gc.GetHeader(key)
}

// SetHeader 设置响应头.
func SetHeader(ctx context.Context, key, value string) {
	if gc, ok := GinContext(ctx); ok {
		gc.Header(key, value)
	}
}

// AddHeader 追加响应头(允许同名多值).
func AddHeader(ctx context.Context, key, value string) {
	if gc, ok := GinContext(ctx); ok {
		gc.Writer.Header().Add(key, value)
	}
}

// ClientIP 返回客户端 IP.
func ClientIP(ctx context.Context) string {
	gc, ok := GinContext(ctx)
	if !ok {
		return ""
	}
	return gc.ClientIP()
}

// Request 返回底层 *http.Request.
func Request(ctx context.Context) *http.Request {
	gc, ok := GinContext(ctx)
	if !ok {
		return nil
	}
	return gc.Request
}

// Cookie 获取 cookie.
func Cookie(ctx context.Context, name string) (string, error) {
	gc, ok := GinContext(ctx)
	if !ok {
		return "", http.ErrNoCookie
	}
	return gc.Cookie(name)
}

// SetCookie 设置 cookie.
func SetCookie(ctx context.Context, name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if gc, ok := GinContext(ctx); ok {
		gc.SetCookie(name, value, maxAge, path, domain, secure, httpOnly)
	}
}

// GetValue 泛型获取中间件值, 类型断言失败返回零值+false.
func GetValue[T any](ctx context.Context, key string) (T, bool) {
	val, exists := Get(ctx, key)
	if !exists {
		var zero T
		return zero, false
	}
	t, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return t, true
}
