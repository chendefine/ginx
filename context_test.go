package ginx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// 编译期验证 *Context 实现 context.Context 接口
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
