package defsrefs

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// GetPerson returns a Person. The spec declares an OpenAPI 3.1 license
// `identifier`, a `$defs` block, and `$ref` references between top-level
// schemas; all must parse without breaking generation.
func (s *TestService) GetPerson(_ context.Context, _ *GetPersonReq) (*GetPersonRsp, error) {
	name := "Ada"
	street := "1 Infinite Loop"
	city := "Cupertino"
	addr := &Address{Street: &street, City: &city}
	return &Person{
		Name:     &name,
		Home:     addr,
		Shipping: addr,
	}, nil
}

var _ ServerInterface = (*TestService)(nil)
