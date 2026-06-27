package validation

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client) {
	r := gin.New()
	RegisterRoutes(r, &TestService{})
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL)
}

func TestCreateValidated_Success(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rating := 2.5
	rsp, err := client.CreateValidated(context.Background(), &CreateValidatedReq{
		Name:          "valid-name",
		Email:         "user@example.com",
		Status:        "active",
		Score:         50,
		Rating:        &rating,
		Tags:          []string{"tag1", "tag2"},
		Website:       strPtr("https://example.com"),
		UUIDField:     strPtr("550e8400-e29b-41d4-a716-446655440000"),
		Ipv4Field:     strPtr("192.168.1.1"),
		Ipv6Field:     strPtr("::1"),
		HostnameField: strPtr("example.com"),
		Phone:         strPtr("+14155552671"),
	})
	if err != nil {
		t.Fatalf("CreateValidated: %v", err)
	}
	if rsp.ID == nil || *rsp.ID != "created-ok" {
		t.Errorf("ID = %v", rsp.ID)
	}
}

func TestCreateValidated_MissingRequired(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	_, err := client.CreateValidated(context.Background(), &CreateValidatedReq{
		Name:   "",
		Email:  "user@example.com",
		Status: "active",
		Score:  50,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var apiErr *ginx.ErrWrap
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *ginx.ErrWrap, got %T: %v", err, err)
	}
}

func TestCreateValidated_InvalidEmail(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	_, err := client.CreateValidated(context.Background(), &CreateValidatedReq{
		Name:   "test",
		Email:  "not-an-email",
		Status: "active",
		Score:  50,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid email")
	}
}

func TestCreateValidated_InvalidEnum(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	_, err := client.CreateValidated(context.Background(), &CreateValidatedReq{
		Name:   "test",
		Email:  "user@example.com",
		Status: "invalid-status",
		Score:  50,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid enum")
	}
}

func TestCreateValidated_ScoreOutOfRange(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	_, err := client.CreateValidated(context.Background(), &CreateValidatedReq{
		Name:   "test",
		Email:  "user@example.com",
		Status: "active",
		Score:  150,
	})
	if err == nil {
		t.Fatal("expected validation error for score > 100")
	}
}

func TestListWithDefaults(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.ListWithDefaults(context.Background(), &ListWithDefaultsReq{})
	if err != nil {
		t.Fatalf("ListWithDefaults: %v", err)
	}
	if rsp.Total == nil || *rsp.Total != 50 {
		t.Errorf("Total = %v, want 50", rsp.Total)
	}
}

func strPtr(s string) *string { return &s }
