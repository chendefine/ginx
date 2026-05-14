package ginx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"resty.dev/v3"
)

// 编译期验证 *Context 实现 context.Context 接口. 业务路径使用标准 context,
// *Context 仅保留用于兼容测试辅助.
var _ context.Context = (*Context)(nil)

func TestContextImplementsContextInterface(t *testing.T) {
	// 运行时也验证一下
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	var _ context.Context = ctx
}

func TestGinContextHelper(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	got, ok := GinContext(ctx)
	if !ok || got != gc {
		t.Fatalf("GinContext() = %v, %v, want %v, true", got, ok, gc)
	}
}

func TestGinContextSupportsDerivedContext(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	derived := context.WithValue(ctx, "trace_id", "t-1")
	got, ok := GinContext(derived)
	if !ok || got != gc {
		t.Fatalf("GinContext(derived) = %v, %v, want %v, true", got, ok, gc)
	}
}

func TestGinContextFromPlainContextWithStoredGinContext(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)

	ctx := context.WithValue(context.Background(), internalGinContextKey, gc)
	got, ok := GinContext(ctx)
	if !ok || got != gc {
		t.Fatalf("GinContext(ctx) = %v, %v, want %v, true", got, ok, gc)
	}
}

func TestAcquiredContextSurvivesRelease(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := acquireContext(gc)
	derived := context.WithValue(ctx, "trace_id", "t-1")

	releaseContext(ctx)

	got, ok := GinContext(derived)
	if !ok || got != gc {
		t.Fatalf("GinContext(derived after release) = %v, %v, want %v, true", got, ok, gc)
	}
	if got := derived.Value("trace_id"); got != "t-1" {
		t.Fatalf("derived value = %v", got)
	}
}

func TestDeadlineDoneErrAndValue(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)

	key := struct{}{}
	base := httptest.NewRequest(http.MethodGet, "/", nil)
	ctxWithValue := context.WithValue(base.Context(), key, "value")
	ctxWithDeadline, cancelDeadline := context.WithDeadline(ctxWithValue, time.Now().Add(time.Minute))
	defer cancelDeadline()
	ctxWithCancel, cancel := context.WithCancel(ctxWithDeadline)
	gc.Request = base.WithContext(ctxWithCancel)

	ctx := newContext(gc)
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("Deadline should report ok=true")
	}
	if time.Until(deadline) <= 0 {
		t.Fatalf("deadline=%v should be in future", deadline)
	}
	if got := ctx.Value(key); got != "value" {
		t.Fatalf("Value=%v", got)
	}

	cancel()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Done channel should be closed after cancel")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("Err=%v", ctx.Err())
	}
}

func TestGetSet(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	Set(ctx, "key1", "value1")
	val, exists := Get(ctx, "key1")
	if !exists {
		t.Fatal("Get returned exists=false for key that was Set")
	}
	if val != "value1" {
		t.Errorf("Get = %v, want %q", val, "value1")
	}

	_, exists = Get(ctx, "nonexistent")
	if exists {
		t.Error("Get returned exists=true for nonexistent key")
	}
}

func TestMustGet(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	Set(ctx, "mykey", 42)
	val := MustGet(ctx, "mykey")
	if val != 42 {
		t.Errorf("MustGet = %v, want 42", val)
	}
}

func TestMustGetPanic(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet should panic for nonexistent key")
		}
	}()
	MustGet(ctx, "nonexistent")
}

func TestGetHeader(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	gc.Request.Header.Set("X-Custom", "hello")

	ctx := newContext(gc)
	if GetHeader(ctx, "X-Custom") != "hello" {
		t.Errorf("GetHeader = %q, want %q", GetHeader(ctx, "X-Custom"), "hello")
	}
}

func TestSetHeaderAndAddHeader(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	SetHeader(ctx, "X-Response", "world")
	AddHeader(ctx, "Set-Cookie", "a=1")
	AddHeader(ctx, "Set-Cookie", "b=2")
	if w.Header().Get("X-Response") != "world" {
		t.Errorf("response header X-Response = %q, want %q", w.Header().Get("X-Response"), "world")
	}
	cookies := w.Header().Values("Set-Cookie")
	if len(cookies) != 2 || cookies[0] != "a=1" || cookies[1] != "b=2" {
		t.Fatalf("cookies=%v", cookies)
	}
}

func TestClientIP(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	gc.Request.RemoteAddr = "192.168.1.1:12345"

	ctx := newContext(gc)
	ip := ClientIP(ctx)
	if ip == "" {
		t.Error("ClientIP() returned empty string")
	}
}

func TestRequest(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	gc.Request = req

	ctx := newContext(gc)
	if Request(ctx) != req {
		t.Error("Request() should return the underlying *http.Request")
	}
}

func TestCookieAndSetCookie(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	gc.Request.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	ctx := newContext(gc)

	val, err := Cookie(ctx, "session")
	if err != nil {
		t.Fatalf("Cookie err=%v", err)
	}
	if val != "abc" {
		t.Fatalf("Cookie=%q", val)
	}

	SetCookie(ctx, "token", "xyz", 60, "/", "example.com", true, true)
	got := w.Header().Get("Set-Cookie")
	if got == "" || !containsAll(got, []string{"token=xyz", "Path=/", "Domain=example.com", "Max-Age=60", "HttpOnly", "Secure"}) {
		t.Fatalf("Set-Cookie=%q", got)
	}
}

func TestGetValueSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	Set(ctx, "uid", int64(12345))
	val, ok := GetValue[int64](ctx, "uid")
	if !ok {
		t.Fatal("GetValue returned false for existing key")
	}
	if val != 12345 {
		t.Errorf("GetValue = %d, want 12345", val)
	}
}

func TestGetValueNotExists(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	val, ok := GetValue[string](ctx, "missing")
	if ok {
		t.Error("GetValue returned true for nonexistent key")
	}
	if val != "" {
		t.Errorf("GetValue zero value = %q, want empty string", val)
	}
}

func TestGetValueWrongType(t *testing.T) {
	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	ctx := newContext(gc)

	Set(ctx, "num", 42) // int
	val, ok := GetValue[string](ctx, "num")
	if ok {
		t.Error("GetValue returned true for wrong type assertion")
	}
	if val != "" {
		t.Errorf("GetValue zero value = %q, want empty string", val)
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func TestSSEStream_Close_Idempotent(t *testing.T) {
	ctx := context.Background()
	es := resty.NewEventSource().SetURL("http://127.0.0.1:1/nonexistent")

	stream := NewSSEStream(ctx, es)

	if err := stream.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestSSEStream_Close_DoesNotBlockWhileConnecting(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	es := resty.NewEventSource().SetURL(srv.URL).SetRetryCount(0)
	stream := NewSSEStream(ctx, es)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("SSE request did not reach test server")
	}

	done := make(chan struct{})
	go func() {
		_ = stream.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close blocked while EventSource was connecting")
	}

	releaseOnce.Do(func() { close(release) })
}

func TestSSEStream_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	es := resty.NewEventSource().SetURL("http://127.0.0.1:1/nonexistent")

	stream := NewSSEStream(ctx, es)

	cancel()
	time.Sleep(20 * time.Millisecond)

	_, err := stream.Recv()
	if err == nil {
		t.Fatal("expected error after context cancel, got nil")
	}
}

func TestSSEStream_RecvAfterClose(t *testing.T) {
	ctx := context.Background()
	es := resty.NewEventSource().SetURL("http://127.0.0.1:1/nonexistent")

	stream := NewSSEStream(ctx, es)
	stream.Close()

	_, err := stream.Recv()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}
}

func TestSSEStream_Integration(t *testing.T) {
	events := []Event{
		{ID: "1", Event: "message", Data: `{"text":"hello"}`},
		{ID: "2", Event: "message", Data: `{"text":"world"}`},
		{ID: "3", Event: "message", Data: "done"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, evt := range events {
			fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", evt.ID, evt.Event, evt.Data)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	es := resty.NewEventSource().SetURL(srv.URL).SetRetryCount(0)
	stream := NewSSEStream(ctx, es)
	defer stream.Close()

	var received []Event
	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv error: %v", err)
		}
		received = append(received, *evt)
	}

	if len(received) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(received))
	}
	for i, evt := range received {
		if evt.ID != events[i].ID {
			t.Errorf("event[%d].ID = %q, want %q", i, evt.ID, events[i].ID)
		}
		if evt.Data != events[i].Data {
			t.Errorf("event[%d].Data = %q, want %q", i, evt.Data, events[i].Data)
		}
	}
}

func TestSSEStream_Integration_CloseEarly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for i := 0; ; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			fmt.Fprintf(w, "id: %d\nevent: message\ndata: tick %d\n\n", i, i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	es := resty.NewEventSource().SetURL(srv.URL).SetRetryCount(0)
	stream := NewSSEStream(ctx, es)

	// Read one event
	evt, err := stream.Recv()
	if err != nil {
		t.Fatalf("first Recv error: %v", err)
	}
	if evt.Data != "tick 0" {
		t.Errorf("expected 'tick 0', got %q", evt.Data)
	}

	// Close mid-stream
	stream.Close()

	// Subsequent Recv should return EOF
	_, err = stream.Recv()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}
}
