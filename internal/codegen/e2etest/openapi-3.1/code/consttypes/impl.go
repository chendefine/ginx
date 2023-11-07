package consttypes

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// CreateConst echoes the parsed request. The OpenAPI 3.1 `const` values are
// enforced by the binding tags (oneof=payment / oneof=3 / oneof=true); an
// invalid value fails validation before the handler runs.
func (s *TestService) CreateConst(_ context.Context, req *CreateConstReq) (*CreateConstRsp, error) {
	accepted := true
	if req.Kind == nil || *req.Kind != "payment" {
		accepted = false
	}
	if req.Retries == nil || *req.Retries != 3 {
		accepted = false
	}
	if req.Active == nil || *req.Active != true {
		accepted = false
	}
	return &CreateConstRsp{Accepted: &accepted}, nil
}

var _ ServerInterface = (*TestService)(nil)
