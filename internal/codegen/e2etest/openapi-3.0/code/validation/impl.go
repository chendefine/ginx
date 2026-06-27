package validation

import (
	"context"
)

type TestService struct{}

func (s *TestService) CreateValidated(_ context.Context, req *CreateValidatedReq) (*CreateValidatedRsp, error) {
	id := "created-ok"
	return &CreateValidatedRsp{ID: &id}, nil
}

func (s *TestService) ListWithDefaults(_ context.Context, req *ListWithDefaultsReq) (*ListWithDefaultsRsp, error) {
	total := 50
	return &ListWithDefaultsRsp{Total: &total}, nil
}

var _ ServerInterface = (*TestService)(nil)
