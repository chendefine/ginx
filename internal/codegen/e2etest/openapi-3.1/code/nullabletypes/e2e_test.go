package nullabletypes

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

func TestGetNullable_NullableTypeArrays(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	rsp, err := client.GetNullable(context.Background(), &GetNullableReq{})
	if err != nil {
		t.Fatalf("GetNullable: %v", err)
	}
	if rsp.ID != 7 {
		t.Errorf("ID = %d, want 7", rsp.ID)
	}
	if rsp.Nickname == nil || *rsp.Nickname != "ace" {
		t.Errorf("Nickname = %v, want \"ace\"", rsp.Nickname)
	}
	if rsp.Age == nil || *rsp.Age != 30 {
		t.Errorf("Age = %v, want 30", rsp.Age)
	}
	if rsp.Flag == nil || !*rsp.Flag {
		t.Errorf("Flag = %v, want true", rsp.Flag)
	}
	if rsp.Score == nil || *rsp.Score != 9.5 {
		t.Errorf("Score = %v, want 9.5", rsp.Score)
	}
	// Nullable array ["array","null"] -> []string (not *any).
	if len(rsp.Tags) != 2 || rsp.Tags[0] != "a" || rsp.Tags[1] != "b" {
		t.Errorf("Tags = %v, want [a b]", rsp.Tags)
	}
}
