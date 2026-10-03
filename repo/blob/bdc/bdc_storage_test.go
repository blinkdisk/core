package bdc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCloseAfterRequest(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		var req Request
		if err := conn.ReadJSON(&req); err != nil {
			t.Errorf("read request failed: %v", err)
			return
		}

		if err := conn.WriteJSON(Response{ResponseID: req.RequestID, URL: "https://example.test/kopia.repository"}); err != nil {
			t.Errorf("write response failed: %v", err)
			return
		}

		// Keep the connection open until the client closes it, as CloudBlink does.
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	storage := &bdcStorage{Options: Options{
		URL:   "ws" + strings.TrimPrefix(server.URL, "http"),
		Token: "test-token",
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := storage.sendRequest(ctx, Request{RequestID: generateRequestID(), Type: msgTypeGetBlob, Key: "kopia.repository"}); err != nil {
		t.Fatalf("sendRequest() error = %v", err)
	}

	closeCtx, closeCancel := context.WithTimeout(ctx, time.Second)
	defer closeCancel()

	if err := storage.Close(closeCtx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case <-storage.responseReaderDone:
	default:
		t.Fatal("Close() returned before the response reader stopped")
	}

	if err := storage.Close(ctx); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCloseBeforeConnecting(t *testing.T) {
	storage := &bdcStorage{Options: Options{URL: ":invalid", Token: "test-token"}}
	ctx := context.Background()

	if err := storage.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := storage.connect(ctx); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("connect() error = %v, want storage closed error", err)
	}
}

func TestCloseWithPendingRequest(t *testing.T) {
	requestReceived := make(chan struct{})
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		var req Request
		if err := conn.ReadJSON(&req); err != nil {
			t.Errorf("read request failed: %v", err)
			return
		}

		close(requestReceived)
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	storage := &bdcStorage{Options: Options{
		URL:   "ws" + strings.TrimPrefix(server.URL, "http"),
		Token: "test-token",
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requestDone := make(chan error, 1)
	go func() {
		_, err := storage.sendRequestAttempt(ctx, Request{RequestID: generateRequestID(), Type: msgTypeGetBlob, Key: "kopia.repository"})
		requestDone <- err
	}()

	select {
	case <-requestReceived:
	case <-ctx.Done():
		t.Fatal("timed out waiting for request")
	}

	if err := storage.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-requestDone:
		if err == nil || !strings.Contains(err.Error(), "connection closed") {
			t.Fatalf("sendRequestAttempt() error = %v, want connection closed error", err)
		}
	case <-ctx.Done():
		t.Fatal("Close() did not unblock the pending request")
	}
}
