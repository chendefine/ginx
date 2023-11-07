package jsonlines

import (
	"context"
	"fmt"

	"github.com/chendefine/ginx"
)

type TestService struct{}

func NewTestService() *TestService { return &TestService{} }

// TailLogs streams log records as newline-delimited JSON (JSON Lines). The
// OpenAPI 3.2 spec declares the response as application/jsonl, which the
// generator turns into a ginx.JSONLines streaming handler + client.
func (s *TestService) TailLogs(_ context.Context, req *TailLogsReq, send ginx.JSONLinesSender) error {
	for i := 1; i <= 3; i++ {
		if err := send(map[string]any{
			"source": req.Source,
			"level":  "info",
			"msg":    fmt.Sprintf("line %d", i),
		}); err != nil {
			return err
		}
	}
	return nil
}

// IngestBatch acknowledges an NDJSON (application/x-ndjson) stream back to the
// caller. Demonstrates a POST with a JSON request body and a JSON Lines
// response.
func (s *TestService) IngestBatch(_ context.Context, req *IngestBatchReq, send ginx.JSONLinesSender) error {
	for i := 0; i < req.Count; i++ {
		if err := send(map[string]any{"ok": true}); err != nil {
			return err
		}
	}
	return nil
}

var _ ServerInterface = (*TestService)(nil)
