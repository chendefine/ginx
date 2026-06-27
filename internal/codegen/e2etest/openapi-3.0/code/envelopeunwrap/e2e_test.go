package envelopeunwrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client, *TestService) {
	svc := NewTestService()
	r := gin.New()
	RegisterRoutes(r, svc)
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL), svc
}

// TestEnvelopeUnwrap_RoundTrip proves the generated client recovers the inner
// business payload for each unwrap variant.
func TestEnvelopeUnwrap_RoundTrip(t *testing.T) {
	srv, client, _ := setupServer()
	defer srv.Close()

	usr, err := client.GetUser(context.Background(), &GetUserReq{})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if usr.ID != 1 || usr.Name != "Alice" {
		t.Errorf("GetUser = %+v, want {ID:1 Name:Alice}", usr)
	}

	prod, err := client.GetProduct(context.Background(), &GetProductReq{})
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if prod.ID != 2 || prod.Name != "Widget" || prod.Price != 9.99 {
		t.Errorf("GetProduct = %+v", prod)
	}

	wrapped, err := client.GetWrapped(context.Background(), &GetWrappedReq{})
	if err != nil {
		t.Fatalf("GetWrapped: %v", err)
	}
	if wrapped.ID != 3 || wrapped.Name != "Bob" {
		t.Errorf("GetWrapped = %+v", wrapped)
	}
}

// TestEnvelopeUnwrap_SingleEnvelopeWire is the double-wrap regression guard: it
// hits the raw HTTP body and asserts ginx wrapped the payload exactly once
// ({code,msg,data:<payload>}), with no nested envelope inside data.
func TestEnvelopeUnwrap_SingleEnvelopeWire(t *testing.T) {
	srv, _, _ := setupServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/user")
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// data carries the business payload directly.
	if !strings.Contains(bodyStr, `"data":{"id":1,"name":"Alice"}`) {
		t.Fatalf("expected single-envelope data payload, got body: %s", bodyStr)
	}
	// A double-wrapped body would have "code" twice (outer + inner envelope).
	// gin's compact JSON renders code as "code":0, so count occurrences of "code":.
	if c := strings.Count(bodyStr, `"code":`); c != 1 {
		t.Fatalf("expected exactly one envelope on the wire, got %d \"code\": occurrences in body: %s", c, bodyStr)
	}
}
