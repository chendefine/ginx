package typemapping

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

func TestListEvents(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListEvents(context.Background(), &ListEventsReq{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if rsp == nil || len(*rsp) != 2 {
		t.Fatalf("expected 2 events, got %v", rsp)
	}
	events := *rsp
	if events[0].ID != 1 {
		t.Errorf("events[0].ID = %d, want 1", events[0].ID)
	}
	if events[0].CreatedAt.IsZero() {
		t.Error("events[0].CreatedAt is zero")
	}
	if events[0].Duration == nil || *events[0].Duration != 3600 {
		t.Errorf("events[0].Duration = %v, want 3600", events[0].Duration)
	}
}
