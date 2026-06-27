package nullabletypes

import (
	"context"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// GetNullable returns a sample exercising OpenAPI 3.1 nullable type arrays
// (["string","null"], ["integer","null"], etc.), which generate pointer fields,
// plus a nullable array (["array","null"]) which generates []string.
func (s *TestService) GetNullable(_ context.Context, _ *GetNullableReq) (*GetNullableRsp, error) {
	nick := "ace"
	age := 30
	flag := true
	score := 9.5
	return &NullableSample{
		ID:       7,
		Nickname: &nick,
		Age:      &age,
		Flag:     &flag,
		Score:    &score,
		Tags:     []string{"a", "b"},
	}, nil
}

var _ ServerInterface = (*TestService)(nil)
