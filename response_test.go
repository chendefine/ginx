package ginx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestFileResponseWriteTo(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	file := t.TempDir() + "/report.txt"
	if err := os.WriteFile(file, []byte("hello file"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	rsp := FileResponse(file, "download.txt")
	if err := rsp.WriteTo(c); err != nil {
		t.Fatalf("WriteTo err=%v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "download.txt") {
		t.Fatalf("content-disposition=%q", got)
	}
	if w.Body.String() != "hello file" {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestStringResponseWithoutArgsKeepsOriginalBody(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	format := "literal %s"
	rsp := StringResponse(http.StatusCreated, format)
	if err := rsp.WriteTo(c); err != nil {
		t.Fatalf("WriteTo err=%v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d", w.Code)
	}
	if w.Body.String() != "literal %s" {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestStringResponseWriteTo(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	rsp := StringResponse(http.StatusOK, "hello %s, age=%d", "bob", 30)
	if err := rsp.WriteTo(c); err != nil {
		t.Fatalf("WriteTo err=%v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if w.Body.String() != "hello bob, age=30" {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestRedirectResponseWriteTo(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	rsp := RedirectResponse(http.StatusFound, "/target")
	if err := rsp.WriteTo(c); err != nil {
		t.Fatalf("WriteTo err=%v", err)
	}

	if w.Code != http.StatusFound {
		t.Fatalf("status=%d", w.Code)
	}
	if w.Header().Get("Location") != "/target" {
		t.Fatalf("location=%q", w.Header().Get("Location"))
	}
}

func TestDataResponseWriteTo(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	rsp := DataResponse(http.StatusOK, "application/octet-stream", []byte("raw bytes here"))
	if err := rsp.WriteTo(c); err != nil {
		t.Fatalf("WriteTo err=%v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("content-type=%q", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "raw bytes here" {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestFileResponseConstructor(t *testing.T) {
	rsp := FileResponse("/tmp/test.pdf", "report.pdf")
	if rsp.FilePath != "/tmp/test.pdf" || rsp.FileName != "report.pdf" {
		t.Fatalf("rsp=%+v", rsp)
	}
}

func TestResponseInterface(t *testing.T) {
	var _ Response = (*FileRsp)(nil)
	var _ Response = (*RedirectRsp)(nil)
	var _ Response = (*StringRsp)(nil)
	var _ Response = (*DataRsp)(nil)
}

type errResponse struct{}

type brokenWriter struct{ gin.ResponseWriter }

func (w *brokenWriter) Write(data []byte) (int, error) { return 0, errors.New("write failed") }

func (errResponse) WriteTo(c *gin.Context) error { return errors.New("write failed") }

func TestHandlerWriteToErrorAddsGinError(t *testing.T) {
	captured := false
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Next()
		captured = len(c.Errors) == 1 && c.Errors[0].Error() == "write failed"
	})
	GET(r, "/x", func(ctx context.Context, req *simpleReq) (*errResponse, error) {
		return &errResponse{}, nil
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if !captured {
		t.Fatalf("expected gin error captured")
	}
}
