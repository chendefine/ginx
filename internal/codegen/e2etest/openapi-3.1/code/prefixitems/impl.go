package prefixitems

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// GetTuple returns a Measurement whose fields are OpenAPI 3.1 prefixItems
// tuples, generated as []any. The heterogeneous positional values round-trip
// through JSON as a generic array.
func (s *TestService) GetTuple(_ context.Context, _ *GetTupleReq) (*GetTupleRsp, error) {
	return &Measurement{
		Coords:  []any{"x", 42},
		Samples: [][]any{{1.5, "a"}},
	}, nil
}

var _ ServerInterface = (*TestService)(nil)
