package webhooks

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

func TestHandleOrderCreated_WebhookReceiver(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	amount := 5
	rsp, err := client.HandleOrderCreated(context.Background(), &HandleOrderCreatedReq{
		OrderID: "ord_123",
		Amount:  &amount,
	})
	if err != nil {
		t.Fatalf("HandleOrderCreated: %v", err)
	}
	if rsp.Received == nil || !*rsp.Received {
		t.Fatalf("expected received=true, got %+v", rsp)
	}
}
