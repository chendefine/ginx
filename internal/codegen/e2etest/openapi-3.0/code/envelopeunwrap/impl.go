package envelopeunwrap

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// GetUser returns the inner business payload. Because the spec's response is the
// ginx envelope {code,msg,data: User}, codegen unwraps it, so GetUserRsp = User
// and ginx's runtime wrapper adds the single envelope at the HTTP boundary.
func (s *TestService) GetUser(_ context.Context, _ *GetUserReq) (*GetUserRsp, error) {
	return &GetUserRsp{ID: 1, Name: "Alice"}, nil
}

// GetProduct exercises an inline data object (not a $ref) inside the envelope.
func (s *TestService) GetProduct(_ context.Context, _ *GetProductReq) (*GetProductRsp, error) {
	return &GetProductRsp{ID: 2, Name: "Widget", Price: 9.99}, nil
}

// GetWrapped exercises a $ref envelope (response is $ref: ApiResponse, whose
// data is $ref: User). The unwrapped type is still User.
func (s *TestService) GetWrapped(_ context.Context, _ *GetWrappedReq) (*GetWrappedRsp, error) {
	return &GetWrappedRsp{ID: 3, Name: "Bob"}, nil
}

var _ ServerInterface = (*TestService)(nil)
