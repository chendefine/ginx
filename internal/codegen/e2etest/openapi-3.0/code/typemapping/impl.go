package typemapping

import (
	"context"
	"time"
)

type TestService struct{}

func (s *TestService) ListEvents(_ context.Context, _ *ListEventsReq) (*ListEventsRsp, error) {
	now := time.Now()
	result := ListEventsRsp{
		{ID: 1, CreatedAt: now, UpdatedAt: &now, Duration: int64Ptr(3600)},
		{ID: 2, CreatedAt: now.Add(-time.Hour)},
	}
	return &result, nil
}

var _ ServerInterface = (*TestService)(nil)

func int64Ptr(v int64) *int64 { return &v }
