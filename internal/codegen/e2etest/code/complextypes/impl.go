package complextypes

import "context"

type TestService struct{}

func (s *TestService) ListPets(_ context.Context, _ *ListPetsReq) (*ListPetsRsp, error) {
	result := ListPetsRsp{
		{ID: 1, Name: "Buddy", Status: PetStatusAvailable, Tags: []string{"friendly", "large"}},
		{ID: 2, Name: "Max", Status: PetStatusSold, Tags: nil},
	}
	return &result, nil
}

var _ ServerInterface = (*TestService)(nil)
