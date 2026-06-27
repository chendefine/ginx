package complextypes

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

func TestListPets(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListPets(context.Background(), &ListPetsReq{})
	if err != nil {
		t.Fatalf("ListPets: %v", err)
	}
	if rsp == nil || len(*rsp) != 2 {
		t.Fatalf("expected 2 pets, got %v", rsp)
	}
	pets := *rsp
	if pets[0].Name != "Buddy" {
		t.Errorf("pets[0].Name = %q, want Buddy", pets[0].Name)
	}
	if pets[0].Status != PetStatusAvailable {
		t.Errorf("pets[0].Status = %q, want available", pets[0].Status)
	}
	if len(pets[0].Tags) != 2 {
		t.Errorf("pets[0].Tags len = %d, want 2", len(pets[0].Tags))
	}
	if pets[1].Status != PetStatusSold {
		t.Errorf("pets[1].Status = %q, want sold", pets[1].Status)
	}
}
