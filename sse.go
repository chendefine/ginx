package ginx

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

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
