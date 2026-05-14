package requestparams

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client) {
	r := gin.New()
	RegisterRoutes(r, &TestService{})
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL)
}

func TestGetUser_PathAndQueryAndHeader(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	fields := "name,email"
	reqID := "req-123"
	sid := "s-123"
	rsp, err := client.GetUser(context.Background(), &GetUserReq{
		UserID:     42,
		Fields:     &fields,
		XRequestID: &reqID,
		Sid:        sid,
	})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != 42 {
		t.Errorf("ID = %v, want 42", rsp.ID)
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
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.UpdateUser(context.Background(), &UpdateUserReq{
		UserID:     1,
		XAuthToken: "token-abc",
		Name:       "NewName",
		Email:      strPtr("new@example.com"),
	})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if rsp.Success == nil || !*rsp.Success {
		t.Error("expected Success=true")
	}
}

func TestCreateUser_EmbedBody(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.CreateUser(context.Background(), &CreateUserReq{
		CreateUserInput: CreateUserInput{Name: "Alice", Email: "alice@example.com"},
		XIdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != 42 {
		t.Errorf("ID = %v, want 42", rsp.ID)
	}
}

func TestSearch_RequiredAndDefaultParams(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.Search(context.Background(), &SearchReq{
		Q: "golang",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if rsp.Total == nil || *rsp.Total != 100 {
		t.Errorf("Total = %v, want 100", rsp.Total)
	}
}

func TestLogin_FormURLEncodedBody(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	remember := true
	rsp, err := client.Login(context.Background(), &LoginReq{
		Username: "alice",
		Password: "secret",
		Remember: &remember,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if rsp.Token == nil || *rsp.Token != "alice:secret:remember" {
		t.Fatalf("Token = %v, want alice:secret:remember", rsp.Token)
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
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListComments(context.Background(), &ListCommentsReq{
		ItemID: "item-99",
	})
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if rsp == nil || len(*rsp) == 0 {
		t.Fatal("expected at least 1 comment")
	}
}

func TestPostScalar_ScalarBody(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.PostScalar(context.Background(), &PostScalarReq{
		Body: "hello",
	})
	if err != nil {
		t.Fatalf("PostScalar: %v", err)
	}
	if rsp.Result == nil || *rsp.Result != "echo: hello" {
		t.Errorf("Result = %v, want 'echo: hello'", rsp.Result)
	}
}

func strPtr(s string) *string { return &s }
