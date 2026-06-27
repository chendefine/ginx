package basictypes

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

func TestGetTypes(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetTypes(context.Background(), &GetTypesReq{})
	if err != nil {
		t.Fatalf("GetTypes: %v", err)
	}
	if rsp.Name != "test" {
		t.Errorf("Name = %q, want test", rsp.Name)
	}
	if rsp.CreatedAt == nil {
		t.Error("CreatedAt is nil")
	}
	if rsp.Email == nil || *rsp.Email != "test@example.com" {
		t.Errorf("Email = %v", rsp.Email)
	}
	if rsp.IPV4 == nil || *rsp.IPV4 != "192.168.1.1" {
		t.Errorf("IPV4 = %v", rsp.IPV4)
	}
	if rsp.Website == nil || *rsp.Website != "https://example.com" {
		t.Errorf("Website = %v", rsp.Website)
	}
}
