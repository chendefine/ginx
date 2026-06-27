package jsonlines

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/chendefine/ginx"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func setupServer() (*httptest.Server, *Client) {
	r := gin.New()
	RegisterRoutes(r, NewTestService())
	srv := httptest.NewServer(r)
	return srv, NewClient(srv.URL)
}

// recvAll drains a JSON Lines stream into a slice of decoded maps.
func recvAll(stream *ginx.JSONLinesStream) ([]map[string]any, error) {
	var out []map[string]any
	for {
		rec, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		var m map[string]any
		if err := json.Unmarshal(rec, &m); err != nil {
			return out, err
		}
		out = append(out, m)
	}
	return out, nil
}

func TestTailLogs_JSONLinesStream(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	stream, err := client.TailLogs(context.Background(), &TailLogsReq{Source: "app"})
	if err != nil {
		t.Fatalf("TailLogs: %v", err)
	}
	defer stream.Close()

	records, err := recvAll(stream)
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	if records[0]["source"] != "app" {
		t.Errorf("records[0] source = %v, want app", records[0]["source"])
	}
	if records[2]["msg"] != "line 3" {
		t.Errorf("records[2] msg = %v, want line 3", records[2]["msg"])
	}
}

func TestIngestBatch_NDJSONResponse(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	// POST with a JSON body; server replies with an NDJSON stream of N acks.
	stream, err := client.IngestBatch(context.Background(), &IngestBatchReq{Count: 2})
	if err != nil {
		t.Fatalf("IngestBatch: %v", err)
	}
	defer stream.Close()

	records, err := recvAll(stream)
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d acks, want 2", len(records))
	}
	for i, r := range records {
		if r["ok"] != true {
			t.Errorf("record %d = %v, want ok=true", i, r)
		}
	}
}
