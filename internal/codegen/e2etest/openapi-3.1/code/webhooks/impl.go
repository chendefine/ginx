package webhooks

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// HandleOrderCreated is an inbound webhook receiver generated from an OpenAPI
// 3.1 top-level `webhooks` entry. The route is synthesized as
// /webhooks/ordercreated.
func (s *TestService) HandleOrderCreated(_ context.Context, req *HandleOrderCreatedReq) (*HandleOrderCreatedRsp, error) {
	received := true
	_ = req.OrderID // required field is bound and non-empty
	return &HandleOrderCreatedRsp{Received: &received}, nil
}

var _ ServerInterface = (*TestService)(nil)
