package bdc

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func TestIsConnectionErrorRecognizesWindowsConnectionReset(t *testing.T) {
	err := errors.Wrap(
		stderrors.New("write tcp [2001:638:50d:1f00::13c]:57516->[2606:4700:3033::ac43:8a6c]:443: wsasend: An existing connection was forcibly closed by the remote host."),
		"failed to send request")

	if !isConnectionError(err) {
		t.Fatal("expected Windows wsasend connection reset to be treated as a connection error")
	}
}

func TestSendRequestReconnectsAfterClosedWebSocket(t *testing.T) {
	var connectionCount atomic.Int32

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		if connectionCount.Add(1) == 1 {
			_ = conn.Close()
			return
		}

		defer conn.Close()

		var req Request
		if err := conn.ReadJSON(&req); err != nil {
			t.Errorf("read request failed: %v", err)
			return
		}

		if req.RequestID == "" {
			t.Error("request ID must be set")
			return
		}

		if err := conn.WriteJSON(Response{
			ResponseID: req.RequestID,
			URL:        "https://example.test/kopia.repository",
		}); err != nil {
			t.Errorf("write response failed: %v", err)
		}
	}))
	defer server.Close()

	storage := &bdcStorage{
		Options: Options{
			URL:   "ws" + strings.TrimPrefix(server.URL, "http"),
			Token: "test-token",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := storage.sendRequest(ctx, Request{
		RequestID: generateRequestID(),
		Type:      msgTypeGetBlob,
		Key:       "kopia.repository",
	})
	if err != nil {
		t.Fatalf("sendRequest() error = %v", err)
	}

	if got, want := resp.URL, "https://example.test/kopia.repository"; got != want {
		t.Fatalf("response URL = %q, want %q", got, want)
	}

	if got := connectionCount.Load(); got < 2 {
		t.Fatalf("connection count = %v, want at least 2", got)
	}
}
