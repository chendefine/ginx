package basictypes

import (
	"context"
	"time"
)

type TestService struct{}

func (s *TestService) GetTypes(_ context.Context, _ *GetTypesReq) (*GetTypesRsp, error) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	return &GetTypesRsp{
		Name:       "test",
		CreatedAt:  &now,
		DateOnly:   strPtr("2024-01-15"),
		RawData:    []byte("raw"),
		BinaryData: []byte("bin"),
		UserID:     strPtr("550e8400-e29b-41d4-a716-446655440000"),
		Website:    strPtr("https://example.com"),
		Email:      strPtr("test@example.com"),
		Host:       strPtr("example.com"),
		IPV4:       strPtr("192.168.1.1"),
		IPV6:       strPtr("::1"),
	}, nil
}

var _ ServerInterface = (*TestService)(nil)

func strPtr(s string) *string { return &s }
