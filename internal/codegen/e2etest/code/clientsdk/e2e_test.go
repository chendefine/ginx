package clientsdk

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client, *TestService) {
	svc := NewTestService()
	r := gin.New()
	RegisterRoutes(r, svc)
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL), svc
}

func TestCreateTokenFormURLEncoded(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	remember := true
	rsp, err := client.CreateToken(context.Background(), &CreateTokenReq{
		Username: "alice",
		Password: "secret",
		Remember: &remember,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if rsp.Token == nil || *rsp.Token != "token-alice" {
		t.Fatalf("Token = %v, want token-alice", rsp.Token)
	}
}

func TestCreateAndGetItem(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()
	ctx := context.Background()

	price := float32(19.99)
	created, err := client.CreateItem(ctx, &CreateItemReq{
		CreateItemInput: CreateItemInput{Name: "Widget", Price: &price},
		XIdempotencyKey: "idem-001",
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if created.ID == nil || *created.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := client.GetItem(ctx, &GetItemReq{ItemID: int64(*created.ID)})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got.Name == nil || *got.Name != "Widget" {
		t.Errorf("Name = %v, want Widget", got.Name)
	}
}

func TestListItems(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _ = client.CreateItem(ctx, &CreateItemReq{
			CreateItemInput: CreateItemInput{Name: "Item"},
			XIdempotencyKey: "idem",
		})
	}

	items, err := client.ListItems(ctx, &ListItemsReq{XTenantID: "t1"})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(*items) != 3 {
		t.Errorf("count = %d, want 3", len(*items))
	}
}

func TestUpdateItem(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()
	ctx := context.Background()

	created, _ := client.CreateItem(ctx, &CreateItemReq{
		CreateItemInput: CreateItemInput{Name: "Old"},
		XIdempotencyKey: "idem",
	})

	newPrice := float32(42.0)
	updated, err := client.UpdateItem(ctx, &UpdateItemReq{
		ItemID: int64(*created.ID),
		Name:   "New",
		Price:  &newPrice,
	})
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if updated.Success == nil || !*updated.Success {
		t.Error("expected Success=true")
	}

	got, _ := client.GetItem(ctx, &GetItemReq{ItemID: int64(*created.ID)})
	if *got.Name != "New" {
		t.Errorf("Name = %q, want New", *got.Name)
	}
}

func TestDeleteItem(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()
	ctx := context.Background()

	created, _ := client.CreateItem(ctx, &CreateItemReq{
		CreateItemInput: CreateItemInput{Name: "Del"},
		XIdempotencyKey: "idem",
	})

	err := client.DeleteItem(ctx, &DeleteItemReq{ItemID: int64(*created.ID)})
	if err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	_, err = client.GetItem(ctx, &GetItemReq{ItemID: int64(*created.ID)})
	if err == nil {
		t.Fatal("expected error after delete")
	}
	var apiErr *ginx.ErrWrap
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *ginx.ErrWrap, got %T", err)
	}
}

func TestGetItemNotFound(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	_, err := client.GetItem(context.Background(), &GetItemReq{ItemID: 99999})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *ginx.ErrWrap
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *ginx.ErrWrap, got %T", err)
	}
	if apiErr.Code != 404 {
		t.Errorf("Code = %d, want 404", apiErr.Code)
	}
}

func TestGetItemDescription(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()
	ctx := context.Background()

	price := float32(9.99)
	created, _ := client.CreateItem(ctx, &CreateItemReq{
		CreateItemInput: CreateItemInput{Name: "Gadget", Price: &price},
		XIdempotencyKey: "idem",
	})

	desc, err := client.GetItemDescription(ctx, &GetItemDescriptionReq{ItemID: int64(*created.ID)})
	if err != nil {
		t.Fatalf("GetItemDescription: %v", err)
	}
	if desc != "Item: Gadget (9.99)" {
		t.Errorf("desc = %q", desc)
	}
}

func TestExportItem(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	data, err := client.ExportItem(context.Background(), &ExportItemReq{ItemID: 1})
	if err != nil {
		t.Fatalf("ExportItem: %v", err)
	}
	if string(data) != "fake-pdf-binary-data" {
		t.Errorf("data = %q", string(data))
	}
}

func TestDeleteNotFound(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()

	err := client.DeleteItem(context.Background(), &DeleteItemReq{ItemID: 99999})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOptionalQueryParam(t *testing.T) {
	srv, client, svc := setupServer()
	defer srv.Close()
	defer svc.Cleanup()
	ctx := context.Background()

	price := float32(5.0)
	created, _ := client.CreateItem(ctx, &CreateItemReq{
		CreateItemInput: CreateItemInput{Name: "OptTest", Price: &price},
		XIdempotencyKey: "idem",
	})

	trueVal := true
	got, err := client.GetItem(ctx, &GetItemReq{ItemID: int64(*created.ID), IncludeDetails: &trueVal})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if *got.Name != "OptTest" {
		t.Errorf("Name = %q, want OptTest", *got.Name)
	}
}
