package clientsdk

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/chendefine/ginx"
)

type itemData struct {
	ID    int64
	Name  string
	Price float32
}

type TestService struct {
	mu       sync.RWMutex
	items    map[int64]*itemData
	nextID   atomic.Int64
	filePath string
}

func NewTestService() *TestService {
	f, _ := os.CreateTemp("", "e2e-export-*.pdf")
	f.Write([]byte("fake-pdf-binary-data"))
	f.Close()
	s := &TestService{items: make(map[int64]*itemData), filePath: f.Name()}
	s.nextID.Store(1)
	return s
}

func (s *TestService) Cleanup() { os.Remove(s.filePath) }

func (s *TestService) ListItems(_ context.Context, _ *ListItemsReq) (*ListItemsRsp, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result ListItemsRsp
	for _, item := range s.items {
		result = append(result, map[string]any{"id": item.ID, "name": item.Name})
	}
	return &result, nil
}

func (s *TestService) CreateItem(_ context.Context, req *CreateItemReq) (*CreateItemRsp, error) {
	id := s.nextID.Add(1) - 1
	s.mu.Lock()
	s.items[id] = &itemData{ID: id, Name: req.Name}
	if req.Price != nil {
		s.items[id].Price = *req.Price
	}
	s.mu.Unlock()
	return &CreateItemRsp{ID: intPtr(int(id))}, nil
}

func (s *TestService) GetItem(_ context.Context, req *GetItemReq) (*GetItemRsp, error) {
	s.mu.RLock()
	item, ok := s.items[req.ItemID]
	s.mu.RUnlock()
	if !ok {
		return nil, ginx.Error(404, "not found").Status(http.StatusNotFound)
	}
	return &GetItemRsp{ID: &item.ID, Name: &item.Name}, nil
}

func (s *TestService) UpdateItem(_ context.Context, req *UpdateItemReq) (*UpdateItemRsp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[req.ItemID]
	if !ok {
		return nil, ginx.Error(404, "not found").Status(http.StatusNotFound)
	}
	item.Name = req.Name
	if req.Price != nil {
		item.Price = *req.Price
	}
	b := true
	return &UpdateItemRsp{Success: &b}, nil
}

func (s *TestService) DeleteItem(_ context.Context, req *DeleteItemReq) (*struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[req.ItemID]; !ok {
		return nil, ginx.Error(404, "not found").Status(http.StatusNotFound)
	}
	delete(s.items, req.ItemID)
	return &struct{}{}, nil
}

func (s *TestService) GetItemDescription(_ context.Context, req *GetItemDescriptionReq) (*ginx.StringRsp, error) {
	s.mu.RLock()
	item, ok := s.items[req.ItemID]
	s.mu.RUnlock()
	if !ok {
		return nil, ginx.Error(404, "not found").Status(http.StatusNotFound)
	}
	return ginx.StringResponse(http.StatusOK, fmt.Sprintf("Item: %s (%.2f)", item.Name, item.Price)), nil
}

func (s *TestService) ExportItem(_ context.Context, _ *ExportItemReq) (*ginx.FileRsp, error) {
	return ginx.FileResponse(s.filePath, "export.pdf"), nil
}

func (s *TestService) RedirectToItems(_ context.Context, _ *RedirectToItemsReq) (*ginx.RedirectRsp, error) {
	return ginx.RedirectResponse(http.StatusFound, "/items"), nil
}

func (s *TestService) UploadItemImage(_ context.Context, _ *UploadItemImageReq) (*UploadItemImageRsp, error) {
	url := "https://cdn.example.com/uploaded.png"
	return &UploadItemImageRsp{URL: &url}, nil
}

var _ ServerInterface = (*TestService)(nil)

func intPtr(v int) *int       { return &v }
func strPtr(s string) *string { return &s }
