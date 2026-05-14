package requestparams

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/chendefine/ginx"
)

type TestService struct{}

func (s *TestService) GetUser(_ context.Context, req *GetUserReq) (*GetUserRsp, error) {
	return &GetUserRsp{ID: &req.UserID}, nil
}

func (s *TestService) UpdateUser(_ context.Context, req *UpdateUserReq) (*UpdateUserRsp, error) {
	b := true
	return &UpdateUserRsp{Success: &b}, nil
}

func (s *TestService) CreateUser(_ context.Context, req *CreateUserReq) (*CreateUserRsp, error) {
	id := int64(42)
	return &CreateUserRsp{ID: &id}, nil
}

func (s *TestService) Search(_ context.Context, req *SearchReq) (*SearchRsp, error) {
	total := 100
	return &SearchRsp{Total: &total}, nil
}

func (s *TestService) UploadFile(_ context.Context, req *UploadFileReq) (*UploadFileRsp, error) {
	url := fmt.Sprintf("https://cdn.example.com/%s", req.File.Filename)
	return &UploadFileRsp{URL: &url}, nil
}

func (s *TestService) UploadBatch(_ context.Context, req *UploadBatchReq) (*UploadBatchRsp, error) {
	count := len(req.Files)
	return &UploadBatchRsp{Count: &count}, nil
}

func (s *TestService) ListComments(_ context.Context, req *ListCommentsReq) (*ListCommentsRsp, error) {
	var result ListCommentsRsp
	result = append(result, map[string]any{"text": fmt.Sprintf("comment for %s", req.ItemID)})
	return &result, nil
}

func (s *TestService) PostScalar(_ context.Context, req *PostScalarReq) (*PostScalarRsp, error) {
	r := fmt.Sprintf("echo: %s", req.Body)
	return &PostScalarRsp{Result: &r}, nil
}

var _ ServerInterface = (*TestService)(nil)
var _ *multipart.FileHeader
var _ = ginx.Error
var _ = http.StatusOK
