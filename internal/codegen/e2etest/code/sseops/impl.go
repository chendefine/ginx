package sseops

import (
	"context"
	"fmt"

	"github.com/chendefine/ginx"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

func (s *TestService) StreamEvents(_ context.Context, req *StreamEventsReq, send ginx.Sender) error {
	for i := 1; i <= 3; i++ {
		if err := send(ginx.Event{
			ID:    fmt.Sprintf("%d", i),
			Event: "message",
			Data:  fmt.Sprintf(`{"channel":"%s","seq":%d}`, req.Channel, i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestService) StreamMetrics(_ context.Context, _ *StreamMetricsReq, send ginx.Sender) error {
	for i := 0; i < 2; i++ {
		if err := send(ginx.Event{
			ID:    fmt.Sprintf("%d", i),
			Event: "metric",
			Data:  fmt.Sprintf(`{"cpu":%d}`, 50+i*10),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestService) StreamNotifications(_ context.Context, req *StreamNotificationsReq, send ginx.Sender) error {
	return send(ginx.Event{
		ID:    "1",
		Event: "notification",
		Data:  fmt.Sprintf(`{"token":"%s"}`, req.XAuthToken),
	})
}

func (s *TestService) StreamRoomMessages(_ context.Context, req *StreamRoomMessagesReq, send ginx.Sender) error {
	return send(ginx.Event{
		ID:    "1",
		Event: "message",
		Data:  fmt.Sprintf(`{"room":"%s","text":"hello"}`, req.RoomID),
	})
}

var _ ServerInterface = (*TestService)(nil)
