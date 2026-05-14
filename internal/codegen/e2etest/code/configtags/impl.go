package configtags

import "context"

type TestService struct{}

func (s *TestService) ListUsers(_ context.Context, _ *ListUsersReq) (*ListUsersRsp, error) {
	var result ListUsersRsp
	result = append(result, map[string]any{"id": "u1"})
	return &result, nil
}

func (s *TestService) ListPets(_ context.Context, _ *ListPetsReq) (*ListPetsRsp, error) {
	var result ListPetsRsp
	result = append(result, map[string]any{"id": "p1"})
	return &result, nil
}

func (s *TestService) HealthCheck(_ context.Context, _ *HealthCheckReq) (*HealthCheckRsp, error) {
	status := "ok"
	return &HealthCheckRsp{Status: &status}, nil
}

func (s *TestService) GetStats(_ context.Context, _ *GetStatsReq) (*GetStatsRsp, error) {
	count := 100
	return &GetStatsRsp{Count: &count}, nil
}

func (s *TestService) GetUntagged(_ context.Context, _ *GetUntaggedReq) (*GetUntaggedRsp, error) {
	data := "untagged"
	return &GetUntaggedRsp{Data: &data}, nil
}

var _ ServerInterface = (*TestService)(nil)

func strPtr(s string) *string { return &s }
