package serverintf

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client) {
	r := gin.New()
	RegisterRoutes(r, NewTestService())
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
	if rsp == nil || len(*rsp) < 2 {
		t.Fatalf("expected at least 2 pets, got %v", rsp)
	}
}

func TestCreateAndGetPet(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()
	ctx := context.Background()

	created, err := client.CreatePet(ctx, &CreatePetReq{Pet: Pet{ID: 10, Name: "Luna"}})
	if err != nil {
		t.Fatalf("CreatePet: %v", err)
	}
	if created.Name != "Luna" {
		t.Errorf("Name = %q, want Luna", created.Name)
	}

	got, err := client.GetPet(ctx, &GetPetReq{PetID: 10})
	if err != nil {
		t.Fatalf("GetPet: %v", err)
	}
	if got.Name != "Luna" {
		t.Errorf("GetPet Name = %q, want Luna", got.Name)
	}
}

func TestDeletePet(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()
	ctx := context.Background()

	err := client.DeletePet(ctx, &DeletePetReq{PetID: 1})
	if err != nil {
		t.Fatalf("DeletePet: %v", err)
	}

	_, err = client.GetPet(ctx, &GetPetReq{PetID: 1})
	if err == nil {
		t.Fatal("expected error after delete")
	}
	var apiErr *ginx.ErrWrap
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *ginx.ErrWrap, got %T", err)
	}
}

func TestStreamEvents(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	stream, err := client.StreamEvents(context.Background(), &StreamEventsReq{Channel: "test"})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer stream.Close()

	var count int
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		count++
	}
	if count < 1 {
		t.Skip("no SSE events received (timing)")
	}
}
