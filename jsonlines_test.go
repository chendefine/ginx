package ginx

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// TestJSONLinesRuntime exercises ginx.JSONLines + JSONLinesStream end to end:
// the server pushes N items via the sender, and the client reads them back as
// newline-delimited JSON until io.EOF. This validates the runtime independent
// of code generation.
func TestJSONLinesRuntime(t *testing.T) {
	type item struct {
		Msg string `json:"msg"`
		N   int    `json:"n"`
	}
	items := []item{{"a", 1}, {"b", 2}, {"c", 3}}

	r := gin.New()
	JSONLines(r, http.MethodPost, "/stream", func(_ context.Context, _ *struct{}, send JSONLinesSender) error {
		for _, it := range items {
			if err := send(it); err != nil {
				return err
			}
		}
		return nil
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/stream", "application/x-ndjson", strings.NewReader(""))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson", ct)
	}

	stream := NewJSONLinesStream(context.Background(), resp.Body)
	var got []item
	for {
		rec, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		var it item
		if err := json.Unmarshal(rec, &it); err != nil {
			t.Fatalf("unmarshal %s: %v", string(rec), err)
		}
		got = append(got, it)
	}
	if len(got) != len(items) {
		t.Fatalf("got %d items, want %d", len(got), len(items))
	}
	for i := range got {
		if got[i] != items[i] {
			t.Errorf("item %d = %+v, want %+v", i, got[i], items[i])
		}
	}
}

// TestJSONLinesStream_BlankLinesSkipped verifies the reader tolerates stray
// blank/whitespace lines (some producers emit a trailing newline).
func TestJSONLinesStream_BlankLinesSkipped(t *testing.T) {
	body := "{\"x\":1}\n\n  \n{\"x\":2}\n"
	stream := NewJSONLinesStream(context.Background(), io.NopCloser(strings.NewReader(body)))
	var xs []int
	for {
		rec, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		var v struct {
			X int `json:"x"`
		}
		if err := json.Unmarshal(rec, &v); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		xs = append(xs, v.X)
	}
	if len(xs) != 2 || xs[0] != 1 || xs[1] != 2 {
		t.Fatalf("got %v", xs)
	}
	// bufio.Reader is used internally; sanity check it's wired.
	_ = bufio.NewReader
}
