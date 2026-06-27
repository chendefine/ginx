package configtags

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

func TestListUsers(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListUsers(context.Background(), &ListUsersReq{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if rsp == nil || len(*rsp) == 0 {
		t.Fatal("expected at least 1 user")
	}
}

func TestListPets(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListPets(context.Background(), &ListPetsReq{})
	if err != nil {
		t.Fatalf("ListPets: %v", err)
	}
	if rsp == nil || len(*rsp) == 0 {
		t.Fatal("expected at least 1 pet")
	}
}

func TestHealthCheck(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.HealthCheck(context.Background(), &HealthCheckReq{})
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if rsp.Status == nil || *rsp.Status != "ok" {
		t.Errorf("Status = %v, want ok", rsp.Status)
	}
}

func TestGetStats(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetStats(context.Background(), &GetStatsReq{})
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if rsp.Count == nil || *rsp.Count != 100 {
		t.Errorf("Count = %v, want 100", rsp.Count)
	}
}

func TestGetUntagged(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetUntagged(context.Background(), &GetUntaggedReq{})
	if err != nil {
		t.Fatalf("GetUntagged: %v", err)
	}
	if rsp.Data == nil || *rsp.Data != "untagged" {
		t.Errorf("Data = %v, want untagged", rsp.Data)
	}
}
