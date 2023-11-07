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
// business payload for each 3.1 envelope variant (nullable inline + allOf).
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

	acct, err := client.GetAccount(context.Background(), &GetAccountReq{})
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.ID != 2 || acct.Name != "Bob" {
		t.Errorf("GetAccount = %+v, want {ID:2 Name:Bob}", acct)
	}
}

// TestEnvelopeUnwrap_SingleEnvelopeWire is the double-wrap regression guard: it
// hits the raw HTTP body and asserts ginx wrapped the payload exactly once
// ({code,msg,data:<payload>}), with no nested envelope inside data. Covers both
// the nullable inline envelope (/user) and the allOf-composed envelope (/account).
func TestEnvelopeUnwrap_SingleEnvelopeWire(t *testing.T) {
	srv, _, _ := setupServer()
	defer srv.Close()

	assertSingleEnvelope := func(path, wantData string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("http.Get %s: %v", path, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// data carries the business payload directly.
		if !strings.Contains(bodyStr, wantData) {
			t.Fatalf("%s: expected single-envelope data payload %q, got body: %s", path, wantData, bodyStr)
		}
		// A double-wrapped body would have "code" twice (outer + inner envelope).
		// gin's compact JSON renders code as "code":0, so count occurrences of "code":.
		if c := strings.Count(bodyStr, `"code":`); c != 1 {
			t.Fatalf("%s: expected exactly one envelope on the wire, got %d \"code\": occurrences in body: %s", path, c, bodyStr)
		}
	}

	assertSingleEnvelope("/user", `"data":{"id":1,"name":"Alice"}`)
	assertSingleEnvelope("/account", `"data":{"id":2,"name":"Bob"}`)
}
