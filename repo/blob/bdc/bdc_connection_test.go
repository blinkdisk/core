package bdc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/blinkdisk/core/internal/gather"
)

func TestBdcStorageWebSocketErrors(t *testing.T) {
	t.Run("AuthenticationFailure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
		defer server.Close()

		opts := &Options{
			URL:   server.URL,
			Token: "invalid-token",
		}

		storage, err := New(context.Background(), opts, true)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer storage.Close(context.Background())

		output := &gather.WriteBuffer{}
		err = storage.GetBlob(context.Background(), "test", 0, -1, output)
		if err == nil {
			t.Error("Expected authentication error")
		}
	})

	t.Run("ConnectionRefused", func(t *testing.T) {
		opts := &Options{
			URL:   "ws://localhost:9999",
			Token: "test-token",
		}

		storage, err := New(context.Background(), opts, true)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer storage.Close(context.Background())

		output := &gather.WriteBuffer{}
		err = storage.GetBlob(context.Background(), "test", 0, -1, output)
		if err == nil {
			t.Error("Expected connection error")
		}
	})
}

func TestBdcStorageReconnection(t *testing.T) {
	t.Run("ReconnectionOnBrokenPipe", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			upgrader := websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool { return true },
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("Failed to upgrade to WebSocket: %v", err)
			}
			defer conn.Close()

			var req Request
			err = conn.ReadJSON(&req)
			if err != nil {
				return
			}

			resp := Response{
				ResponseID: req.RequestID,
				URL:        "https://s3.blinkdisk.com/download/" + req.Key,
			}
			conn.WriteJSON(resp)

			conn.Close()
		}))
		defer server.Close()

		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/download/test-blob" {
				w.Write([]byte("test data"))
			} else {
				http.NotFound(w, r)
			}
		}))
		defer httpServer.Close()

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			upgrader := websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool { return true },
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("Failed to upgrade to WebSocket: %v", err)
			}
			defer conn.Close()

			for {
				var req Request
				err := conn.ReadJSON(&req)
				if err != nil {
					return
				}

				var resp Response
				resp.ResponseID = req.RequestID

				if req.Type == msgTypeGetBlob {
					resp.URL = httpServer.URL + "/download/" + req.Key
				}

				conn.WriteJSON(resp)
			}
		}))
		defer server.Close()

		opts := &Options{
			URL:   server.URL,
			Token: "test-token",
		}

		storage, err := New(context.Background(), opts, true)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer storage.Close(context.Background())

		output := &gather.WriteBuffer{}
		err = storage.GetBlob(context.Background(), "test-blob", 0, -1, output)
		if err != nil {
			t.Errorf("First GetBlob failed: %v", err)
		}

		if string(output.ToByteSlice()) != "test data" {
			t.Errorf("Expected 'test data', got '%s'", string(output.ToByteSlice()))
		}

		output2 := &gather.WriteBuffer{}
		err = storage.GetBlob(context.Background(), "test-blob", 0, -1, output2)
		if err != nil {
			t.Errorf("Second GetBlob failed: %v", err)
		}

		if string(output2.ToByteSlice()) != "test data" {
			t.Errorf("Expected 'test data', got '%s'", string(output2.ToByteSlice()))
		}
	})
}
