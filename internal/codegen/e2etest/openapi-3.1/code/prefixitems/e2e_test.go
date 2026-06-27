package prefixitems

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

func TestGetTuple_TupleRoundTrips(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetTuple(context.Background(), &GetTupleReq{})
	if err != nil {
		t.Fatalf("GetTuple: %v", err)
	}
	// Coords is a heterogeneous tuple [string, int] -> []any{"x", 42}.
	if len(rsp.Coords) != 2 {
		t.Fatalf("Coords len = %d, want 2", len(rsp.Coords))
	}
	if rsp.Coords[0] != "x" {
		t.Errorf("Coords[0] = %v, want \"x\"", rsp.Coords[0])
	}
	// JSON numbers decode to float64 in []any.
	if f, ok := rsp.Coords[1].(float64); !ok || f != 42 {
		t.Errorf("Coords[1] = %v, want 42", rsp.Coords[1])
	}
	// Samples is a nested tuple -> [][]any.
	if len(rsp.Samples) != 1 || len(rsp.Samples[0]) != 2 {
		t.Fatalf("Samples = %+v, want one inner tuple of 2", rsp.Samples)
	}
}
