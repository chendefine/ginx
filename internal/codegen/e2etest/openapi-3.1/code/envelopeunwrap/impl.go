package envelopeunwrap

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// GetUser returns the inner business payload. The spec's 3.1 response is a
// nullable inline envelope {code,msg,data: User} (type arrays), which codegen
// unwraps, so GetUserRsp = User and ginx's runtime wrapper adds the single
// envelope at the HTTP boundary.
func (s *TestService) GetUser(_ context.Context, _ *GetUserReq) (*GetUserRsp, error) {
	return &GetUserRsp{ID: 1, Name: "Alice"}, nil
}

// GetAccount exercises the 3.1 reusable-envelope pattern: the response is an
// allOf composing a nullable Envelope component with data: $ref User. codegen
// flattens the allOf, recognizes the envelope shape, and unwraps to User.
func (s *TestService) GetAccount(_ context.Context, _ *GetAccountReq) (*GetAccountRsp, error) {
	return &GetAccountRsp{ID: 2, Name: "Bob"}, nil
}

var _ ServerInterface = (*TestService)(nil)
