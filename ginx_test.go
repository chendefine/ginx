package ginx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type simpleReq struct{}

type simpleRsp struct {
	Message string `json:"message"`
}

type uriReq struct {
	ID string `uri:"id" binding:"required"`
}

type queryReq struct {
	Page int `form:"page" binding:"required,gt=0"`
}

type gtOnlyQueryReq struct {
	Page int `form:"page" binding:"gt=0"`
}

type ltQueryReq struct {
	Age int `form:"age" binding:"lt=10"`
}

type oneOfQueryReq struct {
	Role string `form:"role" binding:"oneof=admin user"`
}

type emailJSONReq struct {
	Email string `json:"email" binding:"required,email"`
}

type intURIReq struct {
	ID int `uri:"id" binding:"required"`
}

type intHeaderReq struct {
	Token int `header:"X-Token" binding:"required"`
}

type intCookieReq struct {
	SessionID int `cookie:"sid" binding:"required"`
}

type invalidQueryReq struct {
	Page int `form:"page" binding:"required"`
}

type badText string

func (b *badText) UnmarshalText([]byte) error {
	return errors.New("bad text")
}

func mustBadRequestCode(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

type jsonBodyReq struct {
	Name string `json:"name" binding:"required"`
	Age  int    `json:"age"`
}

type uploadReq struct {
	Name string                `form:"name" binding:"required"`
	File *multipart.FileHeader `form:"file" binding:"required"`
}

func newTestEngine(opts ...EngineOption) *Engine {
	return New(opts...)
}

func mustReadRequestBody(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	req := Request(ctx)
	if req == nil || req.Body == nil {
		return nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	return body
}

func mustDecodeBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(body.Bytes(), &data); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	return data
}

func TestCustomSuccessHandler(t *testing.T) {
	e := New(WithSuccessHandler(func(ctx context.Context, data any) (int, any) {
		return http.StatusAccepted, map[string]any{"code": 0, "msg": "ok", "payload": data}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/ok", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "done"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	if body["msg"] != "ok" {
		t.Fatalf("msg=%v", body["msg"])
	}
	payload := body["payload"].(map[string]any)
	if payload["message"] != "done" {
		t.Fatalf("payload=%v", payload)
	}
}

func TestCustomJSONRenderUsesRenderer(t *testing.T) {
	e := New(WithJSONRenderer(func(c *gin.Context, status int, body any) {
		c.Header("X-Renderer", "custom")
		c.JSON(status, body)
	}))
	r := gin.New()
	GET(e.Wrap(r), "/ok", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "pong"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Renderer"); got != "custom" {
		t.Fatalf("renderer header=%q", got)
	}
}

func TestGETWrapsDataByDefault(t *testing.T) {
	r := gin.New()
	GET(r, "/ping", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "pong"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "pong" {
		t.Fatalf("message=%v", data["message"])
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestValidationErrorIsSanitizedByDefault(t *testing.T) {
	r := gin.New()
	POST(r, "/users", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"age":18}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	msg := body["msg"].(string)
	if strings.Contains(msg, "Key:") || strings.Contains(msg, "Error:Field") {
		t.Fatalf("msg not sanitized: %q", msg)
	}
	if msg == "" {
		t.Fatalf("msg should not be empty")
	}
}

func TestValidationErrorUsesTagNameByDefault(t *testing.T) {
	type req struct {
		UserName string `json:"user_name" binding:"required"`
	}

	r := gin.New()
	POST(r, "/users", func(ctx context.Context, req *req) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	body := mustDecodeBody(t, w.Body)
	msg := body["msg"].(string)
	// 应该使用 json tag 名而不是 Go 字段名
	if !strings.Contains(msg, "user_name is required") {
		t.Fatalf("expected tag name in msg, got %q", msg)
	}
	// 确保不包含 Go 字段名
	if strings.Contains(msg, "UserName") {
		t.Fatalf("should not contain Go field name, got %q", msg)
	}
}

func TestPOSTBindsJSONBody(t *testing.T) {
	r := gin.New()
	POST(r, "/items", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.Name}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"alice","age":18}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "alice" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestJSONSuffixContentTypeBindsBody(t *testing.T) {
	r := gin.New()
	POST(r, "/merge", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.Name}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/merge", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/merge-patch+json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "alice" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestJSONCharsetContentTypeBindsBody(t *testing.T) {
	r := gin.New()
	POST(r, "/charset", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.Name}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/charset", strings.NewReader(`{"name":"charset"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "charset" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestIsJSONContentTypeTrimsUppercaseCharset(t *testing.T) {
	if !isJSONContentType("Application/Problem+JSON; Charset=UTF-8") {
		t.Fatalf("expected json content type")
	}
}

func TestDefaultTagAppliesBeforeValidation(t *testing.T) {
	type listReq struct {
		Page int `form:"page" default:"1" binding:"required"`
	}

	r := gin.New()
	GET(r, "/list", func(ctx context.Context, req *listReq) (*simpleRsp, error) {
		return &simpleRsp{Message: strconv.Itoa(req.Page)}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "1" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestWrapDataOverridesEngineNoDataWrap(t *testing.T) {
	e := New(WithDataWrap(false))
	r := gin.New()
	GET(e.Wrap(r), "/wrapped", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "wrapped"}, nil
	}, WrapData())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrapped", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 0 {
		t.Fatalf("code=%v", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["message"] != "wrapped" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestEngineGroupUsesEngineConfiguration(t *testing.T) {
	e := New(WithInvalidArgCode(4012))
	r := gin.New()
	group := e.Group(r, "/api")
	GET(group, "/list", func(ctx context.Context, req *invalidQueryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 4012 {
		t.Fatalf("code=%v", body["code"])
	}
}
func TestMultiSourceBinding(t *testing.T) {
	r := gin.New()
	GET(r, "/users/:id", func(ctx context.Context, req *struct {
		ID    string `uri:"id" binding:"required"`
		Page  int    `form:"page" binding:"required,gt=0"`
		Token string `header:"X-Token" binding:"required"`
		SID   string `cookie:"sid" binding:"required"`
	}) (*simpleRsp, error) {
		return &simpleRsp{Message: req.ID + ":" + req.Token + ":" + req.SID}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/42?page=2", nil)
	req.Header.Set("X-Token", "abc")
	req.AddCookie(&http.Cookie{Name: "sid", Value: "s-1"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestCookieBinding(t *testing.T) {
	r := gin.New()
	GET(r, "/session", func(ctx context.Context, req *struct {
		SessionID string `cookie:"sid" binding:"required"`
		Theme     string `cookie:"theme"`
	}) (*simpleRsp, error) {
		return &simpleRsp{Message: req.SessionID + ":" + req.Theme}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "s-1"})
	req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "s-1:dark" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestCookieBindingValidationErrorUsesCookieName(t *testing.T) {
	r := gin.New()
	GET(r, "/session", func(ctx context.Context, req *struct {
		SessionID string `cookie:"sid" binding:"required"`
	}) (*simpleRsp, error) {
		return &simpleRsp{Message: req.SessionID}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if body["msg"] != "sid is required" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestAlwaysOKWithValidationErrorReturnsHTTP200(t *testing.T) {
	r := gin.New()
	POST(r, "/users", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	}, AlwaysOK())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"age":18}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestMissingValidationReturnsConfiguredCode(t *testing.T) {
	e := newTestEngine(WithInvalidArgCode(4001))
	r := gin.New()
	wrapped := e.Wrap(r)
	GET(wrapped, "/list", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 4001 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestNoDataWrapUsesCustomRenderer(t *testing.T) {
	e := New(WithJSONRenderer(func(c *gin.Context, status int, body any) {
		c.Header("X-Renderer", "raw")
		c.JSON(status, body)
	}))
	r := gin.New()
	GET(e.Wrap(r), "/raw", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "raw"}, nil
	}, NoDataWrap())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/raw", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Renderer"); got != "raw" {
		t.Fatalf("renderer header=%q", got)
	}
}

func TestNoDataWrapRouteOption(t *testing.T) {
	r := gin.New()
	GET(r, "/raw", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "raw"}, nil
	}, NoDataWrap())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/raw", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if body["message"] != "raw" {
		t.Fatalf("message=%v", body["message"])
	}
	if _, ok := body["code"]; ok {
		t.Fatalf("unexpected code field")
	}
}

func TestAlwaysOKRouteOption(t *testing.T) {
	r := gin.New()
	GET(r, "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, Error(1001, "boom").Status(http.StatusServiceUnavailable)
	}, AlwaysOK())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 1001 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestErrWrapUsesStatusCodeWhenAlwaysOKDisabled(t *testing.T) {
	r := gin.New()
	GET(r, "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, Error(2002, "missing").Status(http.StatusNotFound)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestCustomErrorHandler(t *testing.T) {
	e := newTestEngine(WithErrorHandler(func(ctx context.Context, err error) (int, any) {
		return http.StatusConflict, map[string]any{"code": 2001, "msg": err.Error()}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, errors.New("conflict")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 2001 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestJSONRendererUsedForErrorBodies(t *testing.T) {
	e := New(WithJSONRenderer(func(c *gin.Context, status int, body any) {
		c.Header("X-Renderer", "error")
		c.JSON(status, body)
	}))
	r := gin.New()
	GET(e.Wrap(r), "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, errors.New("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Renderer"); got != "error" {
		t.Fatalf("renderer header=%q", got)
	}
}

func TestJSONRendererUsedForValidationBodies(t *testing.T) {
	e := New(WithJSONRenderer(func(c *gin.Context, status int, body any) {
		c.Header("X-Renderer", "validation")
		c.JSON(status, body)
	}))
	r := gin.New()
	GET(e.Wrap(r), "/list", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Renderer"); got != "validation" {
		t.Fatalf("renderer header=%q", got)
	}
}

func TestCustomValidationErrorHandler(t *testing.T) {
	e := newTestEngine(WithValidationErrorHandler(func(ctx context.Context, err error) (int, any) {
		return http.StatusBadRequest, map[string]any{"code": 3001, "msg": "invalid request"}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/list", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 3001 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestPlainErrorUsesConfiguredInternalCode(t *testing.T) {
	e := newTestEngine(WithInternalErrorCode(9002))
	r := gin.New()
	GET(e.Wrap(r), "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, errors.New("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 9002 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestRouteAndEngineInterceptorOrder(t *testing.T) {
	var calls []string
	e := newTestEngine(WithInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		calls = append(calls, "engine-before")
		rsp, err := next()
		calls = append(calls, "engine-after")
		return rsp, err
	}))
	r := gin.New()
	GET(e.Wrap(r), "/x", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		calls = append(calls, "handler")
		return &simpleRsp{Message: "ok"}, nil
	}, RouteInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		calls = append(calls, "route-before")
		rsp, err := next()
		calls = append(calls, "route-after")
		return rsp, err
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	want := []string{"engine-before", "route-before", "handler", "route-after", "engine-after"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestInterceptorWrongReturnTypePanics(t *testing.T) {
	// 拦截器返回错误类型属于编程错误, 框架应在请求处理时 panic 以便尽早暴露.
	r := gin.New()
	GET(r, "/bad", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	}, RouteInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		return map[string]any{"message": "wrong"}, nil
	}))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic but did not panic")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "interceptor returned") {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	r.ServeHTTP(w, req)
}

func TestInterceptorMustReturnMatchingResponseTypePanics(t *testing.T) {
	type rsp struct {
		Message string `json:"message"`
	}

	e := New(WithInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		return gin.H{"message": "wrong"}, nil
	}))
	r := gin.New()
	GET(e.Wrap(r), "/users", func(ctx context.Context, req *simpleReq) (*rsp, error) {
		return &rsp{Message: "ok"}, nil
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic but did not panic")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "interceptor returned") {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.ServeHTTP(w, httpReq)
}

func TestSetDataWrapAffectsDefaultEngine(t *testing.T) {
	old := Default().dataWrap
	defer SetDataWrap(old)
	SetDataWrap(false)

	r := gin.New()
	GET(r, "/raw", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "raw"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/raw", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if body["message"] != "raw" {
		t.Fatalf("body=%v", body)
	}
}

func TestSetInvalidArgumentCodeAffectsDefaultEngine(t *testing.T) {
	old := Default().invalidArgCode
	defer SetInvalidArgumentCode(old)
	SetInvalidArgumentCode(4009)

	r := gin.New()
	GET(r, "/list", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 4009 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestSetInternalServerErrorCodeAffectsDefaultEngine(t *testing.T) {
	old := Default().internalErrorCode
	defer SetInternalServerErrorCode(old)
	SetInternalServerErrorCode(5009)

	r := gin.New()
	GET(r, "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, errors.New("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 5009 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestSetJsonDecoderUseNumberAffectsDefaultEngine(t *testing.T) {
	SetJsonDecoderUseNumber(true)
	defer SetJsonDecoderUseNumber(false)

	type numberReq struct {
		Value any `json:"value" binding:"required"`
	}
	r := gin.New()
	POST(r, "/number", func(ctx context.Context, req *numberReq) (*simpleRsp, error) {
		if _, ok := req.Value.(json.Number); !ok {
			return nil, Error(9001, "not json number")
		}
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/number", strings.NewReader(`{"value":1}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWithJsonDecoderUseNumberIsEngineScoped(t *testing.T) {
	type numberReq struct {
		Value any `json:"value" binding:"required"`
	}

	register := func(e *Engine) *gin.Engine {
		r := gin.New()
		POST(e.Wrap(r), "/number", func(ctx context.Context, req *numberReq) (*simpleRsp, error) {
			_, isJSONNumber := req.Value.(json.Number)
			return &simpleRsp{Message: strconv.FormatBool(isJSONNumber)}, nil
		})
		return r
	}

	withNumber := register(New(WithJsonDecoderUseNumber(true)))
	withoutNumber := register(New())

	call := func(r *gin.Engine) string {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/number", strings.NewReader(`{"value":1}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		body := mustDecodeBody(t, w.Body)
		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatalf("data=%T %v", body["data"], body["data"])
		}
		message, ok := data["message"].(string)
		if !ok {
			t.Fatalf("message=%T %v", data["message"], data["message"])
		}
		return message
	}

	if got := call(withNumber); got != "true" {
		t.Fatalf("withNumber=%s", got)
	}
	if got := call(withoutNumber); got != "false" {
		t.Fatalf("withoutNumber=%s", got)
	}
}

func TestUseNumberIsScopedToEngine(t *testing.T) {
	type req struct {
		Value any `json:"value" binding:"required"`
	}
	type rsp struct {
		Kind string `json:"kind"`
	}

	engineUseNumber := New(WithJsonDecoderUseNumber(true))
	engineDefault := New()
	r := gin.New()

	POST(engineUseNumber.Wrap(r.Group("/use-number")), "/check", func(ctx context.Context, req *req) (*rsp, error) {
		return &rsp{Kind: reflect.TypeOf(req.Value).String()}, nil
	})
	POST(engineDefault.Wrap(r.Group("/default")), "/check", func(ctx context.Context, req *req) (*rsp, error) {
		return &rsp{Kind: reflect.TypeOf(req.Value).String()}, nil
	})

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/use-number/check", want: "json.Number"},
		{path: "/default/check", want: "float64"},
	} {
		w := httptest.NewRecorder()
		httpReq := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"value":1}`))
		httpReq.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, httpReq)
		body := mustDecodeBody(t, w.Body)
		data := body["data"].(map[string]any)
		if data["kind"] != tc.want {
			t.Fatalf("path=%s kind=%v want=%s", tc.path, data["kind"], tc.want)
		}
	}
}

func TestAnyRegistersAllCommonMethods(t *testing.T) {
	r := gin.New()
	Any(r, "/any", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: Request(ctx).Method}, nil
	})

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/any", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("method=%s status=%d", method, w.Code)
		}
	}
}

func TestHandleRegistersSelectedMethods(t *testing.T) {
	r := gin.New()
	Handle(r, []string{http.MethodPost, http.MethodDelete}, "/selected", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/selected", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("method=%s status=%d", method, w.Code)
		}
	}
}

func TestOnRegisterHookReceivesTypes(t *testing.T) {
	var info RegisterInfo
	e := newTestEngine(WithOnRegister(func(i RegisterInfo) { info = i }))
	r := gin.New()
	GET(e.Wrap(r), "/users/:id", func(ctx context.Context, req *uriReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.ID}, nil
	})

	if info.Method != http.MethodGet || info.Path != "/users/:id" {
		t.Fatalf("info=%+v", info)
	}
	if info.ReqType != reflect.TypeOf(uriReq{}) {
		t.Fatalf("reqType=%v", info.ReqType)
	}
	if info.RspType != reflect.TypeOf(simpleRsp{}) {
		t.Fatalf("rspType=%v", info.RspType)
	}
}

func TestEmptyHandlerReturnsWrappedNullData(t *testing.T) {
	r := gin.New()
	GET(r, "/empty", EmptyHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := mustDecodeBody(t, w.Body)
	if _, ok := body["code"]; !ok {
		t.Fatalf("missing code field")
	}
}

func BenchmarkBindJSONVsShouldBindJSON(b *testing.B) {
	payload := []byte(`{"name":"alice","age":18}`)
	type numberReq struct {
		Value any `json:"value" binding:"required"`
	}
	payloadUseNumber := []byte(`{"value":1}`)

	b.Run("ginx_bindJSON", func(b *testing.B) {
		cfg := resolved{}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			gc, _ := gin.CreateTestContext(w)
			gc.Request = httptest.NewRequest(http.MethodPost, "/items", bytes.NewReader(payload))
			gc.Request.Header.Set("Content-Type", "application/json")

			var req jsonBodyReq
			if err := bindJSON(gc, cfg, &req); err != nil {
				b.Fatalf("bindJSON: %v", err)
			}
		}
	})

	b.Run("gin_should_bind_json", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			gc, _ := gin.CreateTestContext(w)
			gc.Request = httptest.NewRequest(http.MethodPost, "/items", bytes.NewReader(payload))
			gc.Request.Header.Set("Content-Type", "application/json")

			var req jsonBodyReq
			if err := gc.ShouldBindJSON(&req); err != nil {
				b.Fatalf("ShouldBindJSON: %v", err)
			}
		}
	})

	b.Run("ginx_bindJSON_use_number", func(b *testing.B) {
		cfg := resolved{jsonDecoderUseNumber: true}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			gc, _ := gin.CreateTestContext(w)
			gc.Request = httptest.NewRequest(http.MethodPost, "/items", bytes.NewReader(payloadUseNumber))
			gc.Request.Header.Set("Content-Type", "application/json")

			var req numberReq
			if err := bindJSON(gc, cfg, &req); err != nil {
				b.Fatalf("bindJSON use number: %v", err)
			}
			if _, ok := req.Value.(json.Number); !ok {
				b.Fatalf("value=%T", req.Value)
			}
		}
	})

	b.Run("gin_should_bind_json_use_number", func(b *testing.B) {
		old := binding.EnableDecoderUseNumber
		binding.EnableDecoderUseNumber = true
		defer func() { binding.EnableDecoderUseNumber = old }()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			gc, _ := gin.CreateTestContext(w)
			gc.Request = httptest.NewRequest(http.MethodPost, "/items", bytes.NewReader(payloadUseNumber))
			gc.Request.Header.Set("Content-Type", "application/json")

			var req numberReq
			if err := gc.ShouldBindJSON(&req); err != nil {
				b.Fatalf("ShouldBindJSON use number: %v", err)
			}
			if _, ok := req.Value.(json.Number); !ok {
				b.Fatalf("value=%T", req.Value)
			}
		}
	})
}

func TestPUTRegistersRoute(t *testing.T) {
	r := gin.New()
	PUT(r, "/items", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "put"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/items", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestPATCHRegistersRoute(t *testing.T) {
	r := gin.New()
	PATCH(r, "/items", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "patch"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/items", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestDELETERegistersRoute(t *testing.T) {
	r := gin.New()
	DELETE(r, "/items", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "delete"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/items", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestHEADRegistersRoute(t *testing.T) {
	r := gin.New()
	HEAD(r, "/items", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "head"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/items", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestOPTIONSRegistersRoute(t *testing.T) {
	r := gin.New()
	OPTIONS(r, "/items", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "options"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/items", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestBuildBindingPlanAcceptsPointerType(t *testing.T) {
	plan := buildBindingPlan(reflect.TypeOf(&jsonBodyReq{}))
	if !plan.hasJSON {
		t.Fatalf("expected json binding plan")
	}
}

func TestBuildBindingPlanNilTypeIsEmpty(t *testing.T) {
	plan := buildBindingPlan(nil)
	if !plan.isEmpty {
		t.Fatalf("expected empty plan")
	}
}

func TestBuildBindingPlanNonStructTypeIsEmpty(t *testing.T) {
	plan := buildBindingPlan(reflect.TypeOf(1))
	if !plan.isEmpty {
		t.Fatalf("expected empty plan")
	}
}

func TestBuildBindingPlanScansNamedNestedStruct(t *testing.T) {
	type nested struct {
		Name string `json:"name"`
	}
	plan := buildBindingPlan(reflect.TypeOf(struct {
		Payload nested
	}{}))
	if !plan.hasJSON {
		t.Fatalf("expected nested json plan")
	}
}

func TestBuildBindingPlanSkipsUnexportedFields(t *testing.T) {
	type req struct {
		visible string
	}
	plan := buildBindingPlan(reflect.TypeOf(req{}))
	if plan.hasJSON {
		t.Fatalf("unexpected json plan for unexported field")
	}
}

func TestBuildBindingPlanTrimsJSONTagOptions(t *testing.T) {
	plan := buildBindingPlan(reflect.TypeOf(struct {
		Name string `json:"name,omitempty"`
	}{}))
	if !plan.hasJSON {
		t.Fatalf("expected json plan")
	}
}

func TestBuildBindingPlanSkipsJSONDashTag(t *testing.T) {
	plan := buildBindingPlan(reflect.TypeOf(struct {
		Name string `json:"-"`
	}{}))
	if plan.hasJSON {
		t.Fatalf("unexpected json plan")
	}
}

func TestBuildBindingPlanHandlesRecursiveStruct(t *testing.T) {
	type node struct {
		Next *node
		Name string `json:"name"`
	}
	plan := buildBindingPlan(reflect.TypeOf(node{}))
	if !plan.hasJSON {
		t.Fatalf("expected recursive json plan")
	}
}

func TestJSONBindingConsumesRequestBodyLikeGin(t *testing.T) {
	r := gin.New()
	POST(r, "/items", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		if string(mustReadRequestBody(t, ctx)) != "" {
			return nil, Error(9002, "body not consumed")
		}
		return &simpleRsp{Message: req.Name}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestJSONBindingTypeErrorUsesBindingErrorResponse(t *testing.T) {
	r := gin.New()
	POST(r, "/items", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.Name}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":123}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 1 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestSanitizeValidationErrorSupportsGtTag(t *testing.T) {
	r := gin.New()
	GET(r, "/list", func(ctx context.Context, req *gtOnlyQueryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list?page=0", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	// 使用 form tag 名
	if body["msg"] != "page must be greater than 0" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestSanitizeValidationErrorSupportsLtTag(t *testing.T) {
	r := gin.New()
	GET(r, "/lt", func(ctx context.Context, req *ltQueryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lt?age=12", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	// 使用 form tag 名
	if body["msg"] != "age must be less than 10" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestSanitizeValidationErrorSupportsOneOfTag(t *testing.T) {
	r := gin.New()
	GET(r, "/role", func(ctx context.Context, req *oneOfQueryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/role?role=guest", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	// 使用 form tag 名
	if body["msg"] != "role must be one of admin user" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestSanitizeValidationErrorSupportsEmailTag(t *testing.T) {
	r := gin.New()
	POST(r, "/email", func(ctx context.Context, req *emailJSONReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/email", strings.NewReader(`{"email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	// 使用 json tag 名
	if body["msg"] != "email must be a valid email" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestSanitizeValidationErrorTagCoverage(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(*gin.Engine)
		request func() *http.Request
		want    string
	}{
		{
			name: "gte",
			setup: func(r *gin.Engine) {
				type req struct {
					Age int `form:"age" binding:"gte=18"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?age=10", nil) },
			want:    "age must be greater than or equal to 18",
		},
		{
			name: "lte",
			setup: func(r *gin.Engine) {
				type req struct {
					Score int `form:"score" binding:"lte=100"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?score=200", nil) },
			want:    "score must be less than or equal to 100",
		},
		{
			name: "eq",
			setup: func(r *gin.Engine) {
				type req struct {
					Version string `form:"version" binding:"eq=v1"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?version=v2", nil) },
			want:    "version must equal v1",
		},
		{
			name: "ne",
			setup: func(r *gin.Engine) {
				type req struct {
					Status string `form:"status" binding:"ne=deleted"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?status=deleted", nil) },
			want:    "status must not equal deleted",
		},
		{
			name: "min",
			setup: func(r *gin.Engine) {
				type req struct {
					Name string `form:"name" binding:"min=3"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?name=ab", nil) },
			want:    "name must be at least 3",
		},
		{
			name: "max",
			setup: func(r *gin.Engine) {
				type req struct {
					Name string `form:"name" binding:"max=5"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?name=toolongname", nil) },
			want:    "name must be at most 5",
		},
		{
			name: "len",
			setup: func(r *gin.Engine) {
				type req struct {
					Code string `form:"code" binding:"len=6"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?code=123", nil) },
			want:    "code length must be exactly 6",
		},
		{
			name: "url",
			setup: func(r *gin.Engine) {
				type req struct {
					Link string `form:"link" binding:"url"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?link=not-a-url", nil) },
			want:    "link must be a valid URL",
		},
		{
			name: "uuid",
			setup: func(r *gin.Engine) {
				type req struct {
					ID string `form:"id" binding:"uuid"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?id=not-uuid", nil) },
			want:    "id must be a valid UUID",
		},
		{
			name: "ip",
			setup: func(r *gin.Engine) {
				type req struct {
					Addr string `form:"addr" binding:"ip"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?addr=999.999.0.1", nil) },
			want:    "addr must be a valid IP address",
		},
		{
			name: "ipv4",
			setup: func(r *gin.Engine) {
				type req struct {
					Addr string `form:"addr" binding:"ipv4"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?addr=::1", nil) },
			want:    "addr must be a valid IPv4 address",
		},
		{
			name: "alpha",
			setup: func(r *gin.Engine) {
				type req struct {
					Name string `form:"name" binding:"alpha"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?name=abc123", nil) },
			want:    "name must contain only letters",
		},
		{
			name: "alphanum",
			setup: func(r *gin.Engine) {
				type req struct {
					Token string `form:"token" binding:"alphanum"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?token=abc-123", nil) },
			want:    "token must contain only letters and numbers",
		},
		{
			name: "numeric",
			setup: func(r *gin.Engine) {
				type req struct {
					Amount string `form:"amount" binding:"numeric"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?amount=12.3x", nil) },
			want:    "amount must be a numeric value",
		},
		{
			name: "boolean",
			setup: func(r *gin.Engine) {
				type req struct {
					Flag string `form:"flag" binding:"boolean"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?flag=maybe", nil) },
			want:    "flag must be a boolean value",
		},
		{
			name: "contains",
			setup: func(r *gin.Engine) {
				type req struct {
					Keyword string `form:"keyword" binding:"contains=go"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?keyword=python", nil) },
			want:    "keyword must contain go",
		},
		{
			name: "startswith",
			setup: func(r *gin.Engine) {
				type req struct {
					Name string `form:"name" binding:"startswith=Mr"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?name=Bob", nil) },
			want:    "name must start with Mr",
		},
		{
			name: "endswith",
			setup: func(r *gin.Engine) {
				type req struct {
					File string `form:"file" binding:"endswith=.go"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?file=main.py", nil) },
			want:    "file must end with .go",
		},
		{
			name: "lowercase",
			setup: func(r *gin.Engine) {
				type req struct {
					Tag string `form:"tag" binding:"lowercase"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?tag=Hello", nil) },
			want:    "tag must be lowercase",
		},
		{
			name: "uppercase",
			setup: func(r *gin.Engine) {
				type req struct {
					Code string `form:"code" binding:"uppercase"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?code=hello", nil) },
			want:    "code must be uppercase",
		},
		{
			name: "json",
			setup: func(r *gin.Engine) {
				type req struct {
					Data string `form:"data" binding:"json"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?data=notjson", nil) },
			want:    "data must be valid JSON",
		},
		{
			name: "unique",
			setup: func(r *gin.Engine) {
				type req struct {
					Tags []string `form:"tags" binding:"unique"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/t?tags=a&tags=a", nil)
			},
			want: "tags must contain unique values",
		},
		{
			name: "default fallback",
			setup: func(r *gin.Engine) {
				type req struct {
					Val string `form:"val" binding:"ascii"`
				}
				GET(r, "/t", func(ctx context.Context, req *req) (*simpleRsp, error) { return nil, nil })
			},
			request: func() *http.Request { return httptest.NewRequest(http.MethodGet, "/t?val=你好", nil) },
			want:    "val must contain only ASCII characters",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			tc.setup(r)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, tc.request())
			body := mustDecodeBody(t, w.Body)
			if body["msg"] != tc.want {
				t.Fatalf("msg=%v, want=%v", body["msg"], tc.want)
			}
		})
	}
}

func TestQueryBindingTypeErrorReturnsBadRequest(t *testing.T) {
	r := gin.New()
	GET(r, "/query", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodGet, "/query?page=bad", nil)
	r.ServeHTTP(w, httpReq)

	mustBadRequestCode(t, w)
}

func TestFormPostBindingTypeErrorReturnsBadRequest(t *testing.T) {
	r := gin.New()
	POST(r, "/form", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("page=bad"))
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	r.ServeHTTP(w, httpReq)

	mustBadRequestCode(t, w)
}

func TestMultipartBindingTypeErrorReturnsBadRequest(t *testing.T) {
	r := gin.New()
	POST(r, "/upload", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("page", "bad"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/upload", &body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, httpReq)

	mustBadRequestCode(t, w)
}

func TestUnknownValidationTagFallsBackToInvalidMessage(t *testing.T) {
	type bindReq struct {
		Name string `form:"name" binding:"min=3"`
	}

	r := gin.New()
	GET(r, "/unknown", func(ctx context.Context, req *bindReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodGet, "/unknown?name=a", nil)
	r.ServeHTTP(w, httpReq)

	body := mustDecodeBody(t, w.Body)
	// 使用 form tag 名
	if body["msg"] != "name must be at least 3" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestSanitizeValidationErrorNonValidationReturnsOriginalError(t *testing.T) {
	if got := sanitizeValidationError(errors.New("boom"), nil); got != "boom" {
		t.Fatalf("msg=%q", got)
	}
}

func TestSanitizeValidationErrorDefaultTagFallsBackToInvalidMessage(t *testing.T) {
	type bindReq struct {
		Name string `form:"name" binding:"startswith=a"`
	}

	r := gin.New()
	GET(r, "/default", func(ctx context.Context, req *bindReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodGet, "/default?name=bob", nil)
	r.ServeHTTP(w, httpReq)

	body := mustDecodeBody(t, w.Body)
	// 应该使用 form tag 名
	if body["msg"] != "name must start with a" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestIsContentTypeAcceptsCharsetSuffix(t *testing.T) {
	if !isContentType("application/x-www-form-urlencoded; charset=utf-8", "application/x-www-form-urlencoded") {
		t.Fatalf("expected content type match")
	}
}

func TestFormPostContentTypeBindsBody(t *testing.T) {
	r := gin.New()
	POST(r, "/form", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: strconv.Itoa(req.Page)}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("page=3"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	data := body["data"].(map[string]any)
	if data["message"] != "3" {
		t.Fatalf("message=%v", data["message"])
	}
}

func TestHeaderBindingTypeErrorReturnsBadRequest(t *testing.T) {
	r := gin.New()
	GET(r, "/header", func(ctx context.Context, req *intHeaderReq) (*simpleRsp, error) {
		return &simpleRsp{Message: strconv.Itoa(req.Token)}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/header", nil)
	req.Header.Set("X-Token", "bad")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCookieBindingTypeErrorReturnsBadRequest(t *testing.T) {
	r := gin.New()
	GET(r, "/cookie", func(ctx context.Context, req *intCookieReq) (*simpleRsp, error) {
		return &simpleRsp{Message: strconv.Itoa(req.SessionID)}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cookie", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "bad"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestURIBindingTypeErrorReturnsBadRequest(t *testing.T) {
	r := gin.New()
	GET(r, "/users/:id", func(ctx context.Context, req *intURIReq) (*simpleRsp, error) {
		return &simpleRsp{Message: strconv.Itoa(req.ID)}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/not-int", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerReturnsNilResponseUsesNullDataField(t *testing.T) {
	r := gin.New()
	GET(r, "/nil", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nil", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if _, ok := body["data"]; !ok {
		t.Fatalf("missing data field: %v", body)
	}
	if body["data"] != nil {
		t.Fatalf("expected null data, got %v", body["data"])
	}
}

func TestHandlerAbortedByInterceptorStopsResponseWrite(t *testing.T) {
	r := gin.New()
	GET(r, "/abort", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "should not write"}, nil
	}, RouteInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		gc, _ := GinContext(ctx)
		gc.AbortWithStatus(http.StatusAccepted)
		return next()
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != "" {
		t.Fatalf("unexpected body=%q", w.Body.String())
	}
}

func TestInterceptorReturningErrorPropagatesToErrorWriter(t *testing.T) {
	r := gin.New()
	GET(r, "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	}, RouteInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		return nil, errors.New("from interceptor")
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if body["msg"] != "from interceptor" {
		t.Fatalf("msg=%v", body["msg"])
	}
}

func TestInterceptorReturningNilResponseWritesWrappedNull(t *testing.T) {
	r := gin.New()
	GET(r, "/nil", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ignored"}, nil
	}, RouteInterceptor(func(ctx context.Context, req any, next func() (any, error)) (any, error) {
		return nil, nil
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nil", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if body["data"] != nil {
		t.Fatalf("expected nil data, got %v", body["data"])
	}
}

func TestSuccessHandlerZeroStatusFallsBackToOK(t *testing.T) {
	e := New(WithSuccessHandler(func(ctx context.Context, data any) (int, any) {
		return 0, map[string]any{"code": 0, "msg": "ok"}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/ok", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "done"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestValidationHandlerWithAlwaysOKForcesHTTP200(t *testing.T) {
	e := New(WithValidationErrorHandler(func(ctx context.Context, err error) (int, any) {
		return http.StatusBadRequest, map[string]any{"code": 3001, "msg": "invalid"}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/list", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	}, AlwaysOK())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestValidationHandlerZeroStatusFallsBackToDefaultBindingError(t *testing.T) {
	e := New(WithValidationErrorHandler(func(ctx context.Context, err error) (int, any) {
		return 0, map[string]any{"code": 9999}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/list", func(ctx context.Context, req *queryReq) (*simpleRsp, error) {
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 1 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestErrorHandlerWithAlwaysOKForcesHTTP200(t *testing.T) {
	e := New(WithErrorHandler(func(ctx context.Context, err error) (int, any) {
		return http.StatusConflict, map[string]any{"code": 2001, "msg": err.Error()}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, errors.New("conflict")
	}, AlwaysOK())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestErrorHandlerZeroStatusFallsBackToDefaultErrorBody(t *testing.T) {
	e := New(WithErrorHandler(func(ctx context.Context, err error) (int, any) {
		return 0, map[string]any{"code": 9999}
	}))
	r := gin.New()
	GET(e.Wrap(r), "/err", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		return nil, errors.New("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)

	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 2 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestSSEHandlerErrorUsesErrorWriter(t *testing.T) {
	r := gin.New()
	SSE(r, "/events", func(ctx context.Context, req *simpleReq, send Sender) error {
		return errors.New("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	if int(body["code"].(float64)) != 2 {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestSSEAcceptsAdditionalRouteOptions(t *testing.T) {
	r := gin.New()
	SSE(r, "/events", func(ctx context.Context, req *simpleReq, send Sender) error {
		return nil
	}, AlwaysOK())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestSSESetsHeadersAndWritesEvent(t *testing.T) {
	r := gin.New()
	SSE(r, "/events", func(ctx context.Context, req *struct{}, send Sender) error {
		return send(Event{Event: "message", Data: gin.H{"ok": true}})
	})

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodGet, "/events", nil)
	r.ServeHTTP(w, httpReq)

	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type=%q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("cache-control=%q", got)
	}
	if got := w.Header().Get("Connection"); got != "keep-alive" {
		t.Fatalf("connection=%q", got)
	}
	if !strings.Contains(w.Body.String(), "event:message") {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestErrWrapIsMatchesSameCode(t *testing.T) {
	if !errors.Is(Error(1001, "a"), Error(1001, "b")) {
		t.Fatalf("expected same code errors to match")
	}
}

func TestErrWrapIsRejectsNonErrWrap(t *testing.T) {
	if errors.Is(Error(1001, "a"), errors.New("a")) {
		t.Fatalf("unexpected match")
	}
}

func TestMultipartUploadBinding(t *testing.T) {
	r := gin.New()
	POST(r, "/upload", func(ctx context.Context, req *uploadReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.Name + ":" + req.File.Filename}, nil
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "avatar")
	part, err := writer.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte("hello")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSSESenderWritesEvent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	sender := newSSESender(c)
	if err := sender(Event{Event: "message", Data: map[string]string{"hello": "world"}}); err != nil {
		t.Fatalf("send err=%v", err)
	}
}

func TestSSESenderReturnsWriterError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Writer = &brokenWriter{ResponseWriter: c.Writer}
	sender := newSSESender(c)
	if err := sender(Event{Event: "message", Data: map[string]string{"hello": "world"}}); err == nil {
		t.Fatalf("expected writer error")
	}
}

func TestSSEDoesNotEmitJSONWrapperAfterStreaming(t *testing.T) {
	r := gin.New()
	SSE(r, "/events", func(ctx context.Context, req *simpleReq, send Sender) error {
		return send(Event{Event: "message", Data: map[string]string{"hello": "world"}})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, `"code":0`) || strings.Contains(body, `"data":`) {
		t.Fatalf("unexpected json wrapper in body=%q", body)
	}
}

func TestSSEWritesEventStream(t *testing.T) {
	r := gin.New()
	SSE(r, "/events", func(ctx context.Context, req *simpleReq, send Sender) error {
		return send(Event{Event: "message", Data: map[string]string{"hello": "world"}})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type=%q", got)
	}
	if body := w.Body.String(); !strings.Contains(body, "event:message") || !strings.Contains(body, `data:{"hello":"world"}`) {
		t.Fatalf("body=%q", body)
	}
}

func TestSSEHandlerSeesRequestContextDone(t *testing.T) {
	r := gin.New()
	observedDone := false
	SSE(r, "/events", func(ctx context.Context, req *simpleReq, send Sender) error {
		select {
		case <-ctx.Done():
			observedDone = true
		default:
		}
		return nil
	})

	baseReq := httptest.NewRequest(http.MethodGet, "/events", nil)
	cancelCtx, cancel := context.WithCancel(baseReq.Context())
	cancel()
	req := baseReq.WithContext(cancelCtx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !observedDone {
		t.Fatalf("expected handler to observe canceled request context")
	}
}

func TestJSONBindingEmptyBodyFallsBackToValidation(t *testing.T) {
	r := gin.New()
	POST(r, "/users", func(ctx context.Context, req *jsonBodyReq) (*simpleRsp, error) {
		return &simpleRsp{Message: req.Name}, nil
	})

	// Content-Type 为 JSON 但 body 为空: 应触发 validator 的 required 错误, 而不是 "invalid request"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := mustDecodeBody(t, w.Body)
	msg := body["msg"].(string)
	if !strings.Contains(msg, "required") {
		t.Fatalf("expected required validation error, got %q", msg)
	}
}

func TestGinContextAccessibleThroughDerivedContext(t *testing.T) {
	r := gin.New()
	GET(r, "/ctx", func(ctx context.Context, req *simpleReq) (*simpleRsp, error) {
		// 模拟业务层对 ctx 再次包装后仍能取到 *gin.Context
		derived := context.WithValue(ctx, struct{ k string }{"test"}, "v")
		gc, ok := GinContext(derived)
		if !ok || gc == nil {
			return nil, Error(9001, "GinContext failed after wrapping")
		}
		return &simpleRsp{Message: "ok"}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ctx", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
