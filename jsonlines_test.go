package ginx

import (
	"context"
	"encoding/json"
	"errors"
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

type countingReadCloser struct {
	io.Reader
	closeCalls int
}

func (r *countingReadCloser) Close() error {
	r.closeCalls++
	return nil
}

func TestJSONLinesWireFormatAndHeaders(t *testing.T) {
	type record struct {
		Message string `json:"message"`
		Value   int    `json:"value"`
	}

	r := gin.New()
	JSONLines(r, http.MethodGet, "/stream", func(_ context.Context, _ *struct{}, send JSONLinesSender) error {
		if err := send(record{Message: "first\nsecond", Value: 1}); err != nil {
			return err
		}
		return send(nil)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if got := w.Header().Get("Connection"); got != "keep-alive" {
		t.Fatalf("Connection = %q, want keep-alive", got)
	}
	if !w.Flushed {
		t.Fatal("sender did not flush streamed records")
	}
	const want = "{\"message\":\"first\\nsecond\",\"value\":1}\nnull\n"
	if got := w.Body.String(); got != want {
		t.Fatalf("wire body = %q, want %q", got, want)
	}
}

func TestJSONLinesBindsAllRequestSources(t *testing.T) {
	type request struct {
		Source string `uri:"source" binding:"required"`
		Follow bool   `form:"follow"`
		Trace  string `header:"X-Trace-ID" binding:"required"`
		Count  int    `json:"count" binding:"required,gt=0"`
	}
	type record struct {
		Source string `json:"source"`
		Follow bool   `json:"follow"`
		Trace  string `json:"trace"`
		Count  int    `json:"count"`
	}

	r := gin.New()
	JSONLines(r, http.MethodPost, "/streams/:source", func(_ context.Context, req *request, send JSONLinesSender) error {
		return send(record{Source: req.Source, Follow: req.Follow, Trace: req.Trace, Count: req.Count})
	})

	req := httptest.NewRequest(http.MethodPost, "/streams/api?follow=true", strings.NewReader(`{"count":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace-ID", "trace-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got record
	if err := json.Unmarshal([]byte(strings.TrimSpace(w.Body.String())), &got); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	want := record{Source: "api", Follow: true, Trace: "trace-123", Count: 2}
	if got != want {
		t.Fatalf("bound record = %+v, want %+v", got, want)
	}
}

func TestJSONLinesBindingErrorDoesNotStartStream(t *testing.T) {
	type request struct {
		Count int `json:"count" binding:"required,gt=0"`
	}

	called := false
	r := gin.New()
	JSONLines(r, http.MethodPost, "/stream", func(_ context.Context, _ *request, _ JSONLinesSender) error {
		called = true
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(`{"count":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if called {
		t.Fatal("handler was called after request validation failed")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON error before stream starts", got)
	}
}

func TestJSONLinesHandlerErrorBeforeFirstRecordUsesHTTPError(t *testing.T) {
	r := gin.New()
	JSONLines(r, http.MethodGet, "/stream", func(_ context.Context, _ *struct{}, _ JSONLinesSender) error {
		return Error(4001, "stream unavailable").Status(http.StatusServiceUnavailable)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON error before stream starts", got)
	}
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Code != 4001 || body.Msg != "stream unavailable" {
		t.Fatalf("error body = %+v", body)
	}
}

func TestJSONLinesEmptyStreamStillHasStreamingHeaders(t *testing.T) {
	r := gin.New()
	JSONLines(r, http.MethodGet, "/stream", func(_ context.Context, _ *struct{}, _ JSONLinesSender) error {
		return nil
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson", got)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("empty stream body = %q", w.Body.String())
	}
}

func TestJSONLinesMarshalErrorBeforeFirstRecordUsesHTTPError(t *testing.T) {
	r := gin.New()
	JSONLines(r, http.MethodGet, "/stream", func(_ context.Context, _ *struct{}, send JSONLinesSender) error {
		return send(make(chan int))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON marshal error", got)
	}
	if strings.Contains(w.Body.String(), "application/x-ndjson") {
		t.Fatalf("unexpected stream metadata in error body: %s", w.Body.String())
	}
}

func TestJSONLinesHandlerErrorAfterFirstRecordDoesNotCorruptStream(t *testing.T) {
	streamErr := errors.New("upstream stream failed")
	var contextErr error
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Next()
		if last := c.Errors.Last(); last != nil {
			contextErr = last.Err
		}
	})
	JSONLines(r, http.MethodGet, "/stream", func(_ context.Context, _ *struct{}, send JSONLinesSender) error {
		if err := send(map[string]any{"sequence": 1}); err != nil {
			return err
		}
		return streamErr
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want committed 200", w.Code)
	}
	if got, want := w.Body.String(), "{\"sequence\":1}\n"; got != want {
		t.Fatalf("wire body = %q, want %q", got, want)
	}
	if !errors.Is(contextErr, streamErr) {
		t.Fatalf("Gin context error = %v, want %v", contextErr, streamErr)
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
}

func TestJSONLinesStream_FinalRecordWithoutNewline(t *testing.T) {
	stream := NewJSONLinesStream(context.Background(), io.NopCloser(strings.NewReader(`{"x":1}`)))
	record, err := stream.Recv()
	if err != nil {
		t.Fatalf("first Recv: %v", err)
	}
	if got := string(record); got != `{"x":1}` {
		t.Fatalf("record = %q", got)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("second Recv error = %v, want io.EOF", err)
	}
}

func TestJSONLinesStreamRecordLargerThanReaderBuffer(t *testing.T) {
	message := strings.Repeat("x", 128*1024)
	body := `{"message":"` + message + `"}` + "\n"
	stream := NewJSONLinesStream(context.Background(), io.NopCloser(strings.NewReader(body)))

	record, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	var got struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(record, &got); err != nil {
		t.Fatalf("unmarshal large record: %v", err)
	}
	if got.Message != message {
		t.Fatalf("large record length = %d, want %d", len(got.Message), len(message))
	}
}

func TestJSONLinesStreamCancelledBeforeRecv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stream := NewJSONLinesStream(ctx, io.NopCloser(strings.NewReader("{\"x\":1}\n")))
	cancel()

	if _, err := stream.Recv(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Recv error = %v, want context.Canceled", err)
	}
}

func TestJSONLinesStreamCloseIsIdempotent(t *testing.T) {
	body := &countingReadCloser{Reader: strings.NewReader("")}
	stream := NewJSONLinesStream(context.Background(), body)

	if err := stream.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if body.closeCalls != 1 {
		t.Fatalf("underlying Close calls = %d, want 1", body.closeCalls)
	}
	if _, err := stream.Recv(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Recv after Close error = %v, want context.Canceled", err)
	}
}
