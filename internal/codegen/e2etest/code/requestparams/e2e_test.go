package requestparams

import (
	"context"
	"net/http/httptest"
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
	rsp, err := client.GetUser(context.Background(), &GetUserReq{
		UserID:     42,
		Fields:     &fields,
		XRequestID: &reqID,
	})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != 42 {
		t.Errorf("ID = %v, want 42", rsp.ID)
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
