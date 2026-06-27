package defsrefs

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

func TestGetPerson_DefsAndRefsResolve(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetPerson(context.Background(), &GetPersonReq{})
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}
	if rsp.Name == nil || *rsp.Name != "Ada" {
		t.Errorf("Name = %v, want Ada", rsp.Name)
	}
	if rsp.Home == nil || rsp.Home.City == nil || *rsp.Home.City != "Cupertino" {
		t.Errorf("Home = %+v, want city Cupertino", rsp.Home)
	}
	// Both home and shipping $ref the same Address schema.
	if rsp.Shipping == nil {
		t.Errorf("Shipping = nil, want Address")
	}
}
