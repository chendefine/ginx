package jsonlinessingle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer(t *testing.T) (*httptest.Server, *Client) {
	t.Helper()
	r := gin.New()
	RegisterRoutes(r, &TestService{})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, NewClient(srv.URL)
}

func receiveAll(t *testing.T, stream *ginx.JSONLinesStream) []map[string]any {
	t.Helper()
	t.Cleanup(func() { _ = stream.Close() })
	var records []map[string]any
	for {
		record, err := stream.Recv()
		if err == io.EOF {
			return records
		}
		if err != nil {
			t.Fatalf("receive JSON Lines record: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(record, &decoded); err != nil {
			t.Fatalf("decode record %q: %v", record, err)
		}
		records = append(records, decoded)
	}
}

func TestSingleFileGeneratedClientAndServer(t *testing.T) {
	_, client := setupServer(t)
	follow := true
	traceID := "trace-single-file"
	stream, err := client.TailLogs(context.Background(), &TailLogsReq{
		Source:   "app worker",
		Follow:   &follow,
		XTraceID: &traceID,
	})
	if err != nil {
		t.Fatalf("TailLogs: %v", err)
	}

	records := receiveAll(t, stream)
	if len(records) != 3 {
		t.Fatalf("record count = %d, want 3", len(records))
	}
	if got := records[0]["source"]; got != "app worker" {
		t.Fatalf("source = %v, want app worker", got)
	}
	if got := records[0]["follow"]; got != true {
		t.Fatalf("follow = %v, want true", got)
	}
	if got := records[0]["trace_id"]; got != traceID {
		t.Fatalf("trace_id = %v, want %s", got, traceID)
	}
	if got := records[2]["message"]; got != "line 3" {
		t.Fatalf("last message = %v, want line 3", got)
	}
}

func TestSingleFileGeneratedPostRoute(t *testing.T) {
	_, client := setupServer(t)
	stream, err := client.IngestBatch(context.Background(), &IngestBatchReq{Count: 2})
	if err != nil {
		t.Fatalf("IngestBatch: %v", err)
	}

	records := receiveAll(t, stream)
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if got := records[1]["sequence"]; got != float64(2) {
		t.Fatalf("second sequence = %v, want 2", got)
	}
}

func TestSingleFileGeneratedResponseHeadersAndFraming(t *testing.T) {
	srv, _ := setupServer(t)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/logs/app/tail", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request generated route: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("NDJSON lines = %d, want 3; body=%q", len(lines), body)
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not valid JSON: %q", i, line)
		}
	}
}

func TestSingleFileGeneratedValidationError(t *testing.T) {
	_, client := setupServer(t)
	_, err := client.IngestBatch(context.Background(), &IngestBatchReq{Count: 0})
	if err == nil {
		t.Fatal("IngestBatch accepted count=0")
	}
	var wrapped *ginx.ErrWrap
	if !errors.As(err, &wrapped) {
		t.Fatalf("error type = %T, want *ginx.ErrWrap", err)
	}
	if wrapped.HttpCode != http.StatusBadRequest {
		t.Fatalf("HTTP status = %d, want 400", wrapped.HttpCode)
	}
}
