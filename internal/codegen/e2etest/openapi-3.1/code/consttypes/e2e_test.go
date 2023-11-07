package consttypes

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
	RegisterRoutes(r, NewTestService())
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL)
}

func TestCreateConst_ValidValuesAccepted(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.CreateConst(context.Background(), &CreateConstReq{
		Kind:    strPtr("payment"),
		Retries: intPtr(3),
		Active:  boolPtr(true),
	})
	if err != nil {
		t.Fatalf("CreateConst: %v", err)
	}
	if rsp.Accepted == nil || !*rsp.Accepted {
		t.Fatalf("expected accepted=true, got %+v", rsp)
	}
}

func TestCreateConst_InvalidConstRejectedByBinding(t *testing.T) {
	srv, _ := setupServer()
	defer srv.Close()

	// kind="refund" violates the const payment -> binding oneof=payment fails.
	body := strings.NewReader(`{"kind":"refund","retries":3,"active":true}`)
	resp, err := http.Post(srv.URL+"/const", "application/json", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 for invalid const, got %d", resp.StatusCode)
	}
}

func strPtr(s string) *string { return &s }
func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
