package querystring

import (
	"context"
	"net/http/httptest"
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

func TestSearch_QuerystringBoundAsQuery(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	// `q` (required) and `limit` are declared `in: querystring`; they bind as
	// ordinary query parameters, so the client sends them on the query string.
	limit := 5
	rsp, err := client.Search(context.Background(), &SearchReq{Q: "golang", Limit: &limit})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if rsp.Q == nil || *rsp.Q != "golang" {
		t.Errorf("Q = %v, want golang", rsp.Q)
	}
	if rsp.Limit == nil || *rsp.Limit != 5 {
		t.Errorf("Limit = %v, want 5", rsp.Limit)
	}
}
