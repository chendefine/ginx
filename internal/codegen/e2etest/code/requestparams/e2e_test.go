package requestparams

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() *httptest.Server {
	r := gin.New()
	RegisterRoutes(r, &TestService{})
	srv := httptest.NewServer(r)
	return srv
}

func TestGetUser_PathAndQueryAndHeader(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/users/42?fields=name,email", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Request-ID", "req-123")
	req.AddCookie(&http.Cookie{Name: "sid", Value: "s-123"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), `"id":42`) {
		t.Fatalf("body=%s, want id 42", string(body))
	}
}

func TestGetUser_CookieParamRequiredByServer(t *testing.T) {
	r := gin.New()
	RegisterRoutes(r, &TestService{})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.Header.Set("X-Request-ID", "req-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateUser_PathAndBodyAndHeader(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, srv.URL+"/users/1", strings.NewReader(`{"name":"NewName","email":"new@example.com"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "token-abc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"success":true`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestCreateUser_EmbedBody(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "idem-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"id":42`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestSearch_RequiredAndDefaultParams(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/search?q=golang")
	if err != nil {
		t.Fatalf("get search: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"total":100`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestLogin_FormURLEncodedBody(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/login", strings.NewReader("username=alice&password=secret&remember=true"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "alice:secret:remember") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestLogin_FormTagStillBindsQueryString(t *testing.T) {
	r := gin.New()
	RegisterRoutes(r, &TestService{})

	req := httptest.NewRequest(http.MethodPost, "/login?username=alice&password=secret&remember=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "alice:secret:remember") {
		t.Fatalf("body=%s, want token from query-bound form fields", w.Body.String())
	}
}

func TestListComments_PathLevelParam(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/items/item-99/comments")
	if err != nil {
		t.Fatalf("get comments: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "comment for item-99") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestPostScalar_ScalarBody(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/body-scalar", strings.NewReader(`{"body":"hello"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "echo: hello") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestUploadFile_MultipartServerBinding(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("description", "avatar"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	file, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := file.Write([]byte("png")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/upload", &body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(respBody), "avatar.png") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestUploadBatch_MultipartFileArrayBinding(t *testing.T) {
	srv := setupServer()
	defer srv.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range []string{"a.txt", "b.txt"} {
		file, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("create file: %v", err)
		}
		if _, err := file.Write([]byte(name)); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/upload/batch", &body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(respBody), `"count":2`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(respBody))
	}
}
