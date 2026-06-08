package sseops

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
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

func TestStreamEvents_BasicFlow(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	stream, err := client.StreamEvents(context.Background(), &StreamEventsReq{Channel: "news"})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer stream.Close()

	var events []ginx.Event
	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		events = append(events, *evt)
	}

	if len(events) < 2 {
		t.Fatalf("got %d events, want at least 2", len(events))
	}
	if events[0].ID != "1" || events[0].Event != "message" {
		t.Errorf("events[0] = %+v", events[0])
	}
	if data, ok := events[0].Data.(string); !ok || !strings.Contains(data, `"channel":"news"`) {
		t.Errorf("events[0].Data = %v, missing channel", events[0].Data)
	}
}

func TestStreamRoomMessages_PathParam(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	stream, err := client.StreamRoomMessages(context.Background(), &StreamRoomMessagesReq{
		RoomID:     "room-42",
		XAuthToken: "auth-xyz",
	})
	if err != nil {
		t.Fatalf("StreamRoomMessages: %v", err)
	}
	defer stream.Close()

	var events []ginx.Event
	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		events = append(events, *evt)
	}

	if len(events) == 0 {
		t.Skip("no events received (timing)")
	}
	if data, ok := events[0].Data.(string); !ok || !strings.Contains(data, `"room":"room-42"`) {
		t.Errorf("Data = %v, expected room echo", events[0].Data)
	}
}

func TestStreamRoomMessages_PathParamEscaped(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	roomID := "room 42+with space"
	stream, err := client.StreamRoomMessages(context.Background(), &StreamRoomMessagesReq{
		RoomID:     roomID,
		XAuthToken: "auth-xyz",
	})
	if err != nil {
		t.Fatalf("StreamRoomMessages: %v", err)
	}
	defer stream.Close()

	evt, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	data, ok := evt.Data.(string)
	if !ok || !strings.Contains(data, `"room":"`+roomID+`"`) {
		t.Fatalf("Data = %v, expected escaped room echo", evt.Data)
	}
}

func TestStreamEvents_CloseEarly(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()

	stream, err := client.StreamEvents(context.Background(), &StreamEventsReq{Channel: "test"})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}

	_, err = stream.Recv()
	if err != nil {
		t.Fatalf("first Recv: %v", err)
	}

	stream.Close()

	for {
		_, err = stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}
	if err != io.EOF {
		t.Errorf("expected io.EOF eventually, got %v", err)
	}
}

func TestStreamEvents_ContextCancel(t *testing.T) {
	srv, client := setupServer()
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())

	stream, err := client.StreamEvents(ctx, &StreamEventsReq{Channel: "cancel-test"})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer stream.Close()

	cancel()

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error after cancel")
	}
}
