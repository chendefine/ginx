package querystring

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// Search echoes back the query parameters. The spec declares them as
// `in: querystring` (OpenAPI 3.2), which ginx normalizes to ordinary query
// parameters before generation.
func (s *TestService) Search(_ context.Context, req *SearchReq) (*SearchRsp, error) {
	limit := 10
	if req.Limit != nil {
		limit = *req.Limit
	}
	return &SearchRsp{Q: &req.Q, Limit: &limit}, nil
}

var _ ServerInterface = (*TestService)(nil)
