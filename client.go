package ginx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"resty.dev/v3"
)

type dataWrapper struct {
	Code *int            `json:"code"`
	Msg  *string         `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func parseDataWrapper(body []byte) (dataWrapper, bool, error) {
	var wrapper dataWrapper
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return dataWrapper{}, false, err
	}
	if wrapper.Code == nil {
		return dataWrapper{}, false, nil
	}
	if wrapper.Msg == nil && wrapper.Data == nil {
		return dataWrapper{}, false, nil
	}
	return wrapper, true, nil
}

func wrapperMsg(wrapper dataWrapper) string {
	if wrapper.Msg == nil {
		return ""
	}
	return *wrapper.Msg
}

// ParseResponse 解析 HTTP 响应体, 兼容 DataWrap 和 NoDataWrap 两种模式.
//   - HTTP 错误 + 空 body → 返回 *ErrWrap{HttpCode}
//   - body 为 {code, msg, data} 格式且 code != 0 → 返回 *ErrWrap 业务错误
//   - body 为 {code:0, data:...} 格式 → 从 data 字段反序列化 result
//   - body 非 wrapper 格式 + HTTP 错误 → 返回 *ErrWrap{HttpCode, Msg: body}
//   - body 非 wrapper 格式 + HTTP 成功 → 直接反序列化 body 到 result
func ParseResponse(statusCode int, body []byte, result any) error {
	if len(body) == 0 {
		if statusCode >= 400 {
			return &ErrWrap{HttpCode: statusCode}
		}
		return nil
	}

	if wrapper, ok, err := parseDataWrapper(body); err == nil && ok {
		if *wrapper.Code != 0 {
			return &ErrWrap{Code: *wrapper.Code, Msg: wrapperMsg(wrapper), HttpCode: statusCode}
		}
		if result != nil && wrapper.Data != nil {
			return json.Unmarshal(wrapper.Data, result)
		}
		return nil
	}

	if statusCode >= 400 {
		return &ErrWrap{Code: -1, Msg: string(body), HttpCode: statusCode}
	}
	if result != nil {
		return json.Unmarshal(body, result)
	}
	return nil
}

// SSEStream provides a pull-based interface for consuming Server-Sent Events.
// It bridges resty's callback-based SSESource into a gRPC-style Recv() pattern.
//
// Internally uses 2 goroutines: one for the blocking es.Get() call, one for
// context-cancellation cleanup. Both are guaranteed to exit when the stream
// ends (naturally or via Close/context cancel).
//
// Callers should call Close() when done, or rely on context cancellation to
// release resources.
type SSEStream struct {
	es     *resty.SSESource
	ch     chan Event
	errCh  chan error
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

// NewSSEStream creates an SSEStream from a configured resty SSESource.
// The caller should have already set URL, headers, etc. on the SSESource.
//
// The stream is safe against leaks: if the parent context is cancelled,
// the underlying connection is closed automatically even without an
// explicit Close() call.
func NewSSEStream(ctx context.Context, es *resty.SSESource) *SSEStream {
	ctx, cancel := context.WithCancel(ctx)
	s := &SSEStream{
		es:     es,
		ch:     make(chan Event, 16),
		errCh:  make(chan error, 1),
		ctx:    ctx,
		cancel: cancel,
	}

	es.OnMessage(func(e any) {
		event, ok := e.(*resty.SSE)
		if !ok {
			return
		}
		select {
		case s.ch <- Event{ID: event.ID, Event: event.Name, Data: event.Data}:
		case <-ctx.Done():
		}
	}, nil)

	es.OnError(func(err error) {
		select {
		case s.errCh <- err:
		default:
		}
	})

	es.OnRequestFailure(func(err error, res *http.Response) {
		if res != nil {
			res.Body.Close()
		}
		select {
		case s.errCh <- err:
		default:
		}
	})

	streamDone := make(chan struct{})

	go func() {
		err := es.Get()
		if err != nil {
			select {
			case s.errCh <- err:
			default:
			}
		}
		close(s.ch)
		close(streamDone)
	}()

	go func() {
		select {
		case <-ctx.Done():
			s.closeES()
		case <-streamDone:
		}
	}()

	return s
}

// Recv blocks until the next event arrives, an error occurs, or the stream is closed.
// Returns io.EOF when the server closes the connection normally or after Close() is called.
// Events are always fully drained before errors are surfaced.
func (s *SSEStream) Recv() (*Event, error) {
	// 优先非阻塞读取已缓冲事件, 确保 context 取消后仍能逐个 drain.
	select {
	case evt, ok := <-s.ch:
		if !ok {
			select {
			case err := <-s.errCh:
				return nil, err
			default:
				return nil, io.EOF
			}
		}
		return &evt, nil
	default:
	}

	// 无缓冲事件时阻塞等待.
	select {
	case evt, ok := <-s.ch:
		if !ok {
			select {
			case err := <-s.errCh:
				return nil, err
			default:
				return nil, io.EOF
			}
		}
		return &evt, nil
	case <-s.ctx.Done():
		select {
		case evt, ok := <-s.ch:
			if ok {
				return &evt, nil
			}
			select {
			case err := <-s.errCh:
				return nil, err
			default:
			}
		default:
		}
		return nil, io.EOF
	}
}

// Close terminates the SSE connection and releases resources. Safe to call multiple times.
func (s *SSEStream) Close() error {
	s.cancel()
	s.closeES()
	return nil
}

func (s *SSEStream) closeES() {
	s.once.Do(func() {
		go s.es.Close()
	})
}
