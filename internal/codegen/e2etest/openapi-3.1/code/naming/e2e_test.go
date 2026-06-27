package naming

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

func TestListHTTPAPIEndpoints(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListHTTPAPIEndpoints(context.Background(), &ListHTTPAPIEndpointsReq{
		APIID: "my-api",
	})
	if err != nil {
		t.Fatalf("ListHTTPAPIEndpoints: %v", err)
	}
	if rsp == nil || len(*rsp) == 0 {
		t.Fatal("expected at least 1 endpoint")
	}
	ep, ok := (*rsp)[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", (*rsp)[0])
	}
	if ep["url"] != "https://api.example.com/my-api" {
		t.Errorf("url = %v", ep["url"])
	}
}

func TestGetNoOperationID(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetNoOperationIDResourceID(context.Background(), &GetNoOperationIDResourceIDReq{
		ResourceID: "res-123",
	})
	if err != nil {
		t.Fatalf("GetNoOperationIDResourceID: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != "res-123" {
		t.Errorf("ID = %v, want res-123", rsp.ID)
	}
}

func TestPostNoOperationID(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.PostNoOperationIDResourceID(context.Background(), &PostNoOperationIDResourceIDReq{
		ResourceID: "res-456",
		Data:       strPtr("payload"),
	})
	if err != nil {
		t.Fatalf("PostNoOperationIDResourceID: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != "res-456" {
		t.Errorf("ID = %v, want res-456", rsp.ID)
	}
}

func strPtr(s string) *string { return &s }
