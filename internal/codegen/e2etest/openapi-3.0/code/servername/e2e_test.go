package servername

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

func TestListOrders(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListOrders(context.Background(), &ListOrdersReq{})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if rsp == nil || len(*rsp) != 2 {
		t.Fatalf("expected 2 orders, got %v", rsp)
	}
}

func TestCreateOrder(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.CreateOrder(context.Background(), &CreateOrderReq{
		ProductID: "prod-1",
		Quantity:  intPtr(3),
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != "new-order" {
		t.Errorf("ID = %v, want new-order", rsp.ID)
	}
}

func intPtr(v int) *int { return &v }
