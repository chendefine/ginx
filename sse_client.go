package ginx

import (
	"context"
	"io"
	"net/http"
	"sync"

	"resty.dev/v3"
)

// SSEStream provides a pull-based interface for consuming Server-Sent Events.
// It bridges resty's callback-based EventSource into a gRPC-style Recv() pattern.
//
// Internally uses 2 goroutines: one for the blocking es.Get() call, one for
// context-cancellation cleanup. Both are guaranteed to exit when the stream
// ends (naturally or via Close/context cancel).
//
// Callers should call Close() when done, or rely on context cancellation to
// release resources.
type SSEStream struct {
	es     *resty.EventSource
	ch     chan Event
	errCh  chan error
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

// NewSSEStream creates an SSEStream from a configured resty EventSource.
// The caller should have already set URL, headers, etc. on the EventSource.
//
// The stream is safe against leaks: if the parent context is cancelled,
// the underlying connection is closed automatically even without an
// explicit Close() call.
func NewSSEStream(ctx context.Context, es *resty.EventSource) *SSEStream {
	ctx, cancel := context.WithCancel(ctx)
	s := &SSEStream{
		es:     es,
		ch:     make(chan Event, 16),
		errCh:  make(chan error, 1),
		ctx:    ctx,
		cancel: cancel,
	}

	es.OnMessage(func(e any) {
		event := e.(*resty.Event)
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
		// Drain any remaining buffered events before returning EOF.
		select {
		case evt, ok := <-s.ch:
			if ok {
				return &evt, nil
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
