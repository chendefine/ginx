package jsonlinessingle

import (
	"context"
	"fmt"

	"github.com/chendefine/ginx"
)

type TestService struct{}

func (s *TestService) TailLogs(_ context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error {
	follow := false
	if req.Follow != nil {
		follow = *req.Follow
	}
	traceID := ""
	if req.XTraceID != nil {
		traceID = *req.XTraceID
	}
	for i := 1; i <= 3; i++ {
		if err := send(map[string]any{
			"source":   req.Source,
			"follow":   follow,
			"trace_id": traceID,
			"message":  fmt.Sprintf("line %d", i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestService) IngestBatch(_ context.Context, req *IngestBatchReq, send ginx.JSONLinesSender) error {
	for i := 1; i <= req.Count; i++ {
		if err := send(map[string]any{"ok": true, "sequence": i}); err != nil {
			return err
		}
	}
	return nil
}

var _ ServerInterface = (*TestService)(nil)
