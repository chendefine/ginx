package ginx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"resty.dev/v3"
)

func TestSSEStream_Close_Idempotent(t *testing.T) {
	ctx := context.Background()
	es := resty.NewEventSource().SetURL("http://127.0.0.1:1/nonexistent")

	stream := NewSSEStream(ctx, es)

	if err := stream.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestSSEStream_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	es := resty.NewEventSource().SetURL("http://127.0.0.1:1/nonexistent")

	stream := NewSSEStream(ctx, es)

	cancel()
	time.Sleep(20 * time.Millisecond)

	_, err := stream.Recv()
	if err == nil {
		t.Fatal("expected error after context cancel, got nil")
	}
}

func TestSSEStream_RecvAfterClose(t *testing.T) {
	ctx := context.Background()
	es := resty.NewEventSource().SetURL("http://127.0.0.1:1/nonexistent")

	stream := NewSSEStream(ctx, es)
	stream.Close()

	_, err := stream.Recv()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}
}

func TestSSEStream_Integration(t *testing.T) {
	events := []Event{
		{ID: "1", Event: "message", Data: `{"text":"hello"}`},
		{ID: "2", Event: "message", Data: `{"text":"world"}`},
		{ID: "3", Event: "message", Data: "done"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, evt := range events {
			fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", evt.ID, evt.Event, evt.Data)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	es := resty.NewEventSource().SetURL(srv.URL).SetRetryCount(0)
	stream := NewSSEStream(ctx, es)
	defer stream.Close()

	var received []Event
	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv error: %v", err)
		}
		received = append(received, *evt)
	}

	if len(received) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(received))
	}
	for i, evt := range received {
		if evt.ID != events[i].ID {
			t.Errorf("event[%d].ID = %q, want %q", i, evt.ID, events[i].ID)
		}
		if evt.Data != events[i].Data {
			t.Errorf("event[%d].Data = %q, want %q", i, evt.Data, events[i].Data)
		}
	}
}

func TestSSEStream_Integration_CloseEarly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for i := 0; ; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			fmt.Fprintf(w, "id: %d\nevent: message\ndata: tick %d\n\n", i, i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	es := resty.NewEventSource().SetURL(srv.URL).SetRetryCount(0)
	stream := NewSSEStream(ctx, es)

	// Read one event
	evt, err := stream.Recv()
	if err != nil {
		t.Fatalf("first Recv error: %v", err)
	}
	if evt.Data != "tick 0" {
		t.Errorf("expected 'tick 0', got %q", evt.Data)
	}

	// Close mid-stream
	stream.Close()

	// Subsequent Recv should return EOF
	_, err = stream.Recv()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}
}
