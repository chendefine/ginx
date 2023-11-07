package responsevariants

import (
	"context"
	"io"
	"net/http"
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

func TestResponseVariantClientBranches(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	created, err := client.CreateJob(context.Background(), &CreateJobReq{Outcome: "201"})
	if err != nil {
		t.Fatalf("CreateJob 201: %v", err)
	}
	if created.StatusCode() != http.StatusCreated {
		t.Fatalf("201 status = %d", created.StatusCode())
	}
	createdBody, ok := created.As201()
	if !ok || createdBody.ID != "resource-1" {
		t.Fatalf("201 branch = %#v, %v", createdBody, ok)
	}
	if _, ok := created.As202(); ok {
		t.Fatal("201 response unexpectedly matched 202 branch")
	}

	accepted, err := client.CreateJob(context.Background(), &CreateJobReq{Outcome: "202"})
	if err != nil {
		t.Fatalf("CreateJob 202: %v", err)
	}
	acceptedBody, ok := accepted.As202()
	if accepted.StatusCode() != http.StatusAccepted || !ok || acceptedBody.JobID != "job-1" {
		t.Fatalf("202 response = %#v, branch=%#v, ok=%v", accepted, acceptedBody, ok)
	}
}

func TestResponseVariant204HasNoBody(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	variant, err := client.CreateJob(context.Background(), &CreateJobReq{Outcome: "204"})
	if err != nil {
		t.Fatalf("CreateJob 204: %v", err)
	}
	if variant.StatusCode() != http.StatusNoContent {
		t.Fatalf("204 client status = %d", variant.StatusCode())
	}
	if _, ok := variant.As201(); ok {
		t.Fatal("204 response unexpectedly matched 201 branch")
	}
	if _, ok := variant.As202(); ok {
		t.Fatal("204 response unexpectedly matched 202 branch")
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/jobs?outcome=204", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent || len(body) != 0 {
		t.Fatalf("204 response status=%d body=%q", resp.StatusCode, body)
	}
}
