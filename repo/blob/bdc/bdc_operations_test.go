package bdc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/blinkdisk/core/internal/gather"
	"github.com/blinkdisk/core/repo/blob"
)

func TestBdcStorageOperations(t *testing.T) {
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

		for {
			var req Request
			err := conn.ReadJSON(&req)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					t.Errorf("WebSocket error: %v", err)
				}
				return
			}

			var resp Response
			resp.ResponseID = req.RequestID

			switch req.Type {
			case msgTypePutBlob:
				resp.URL = "https://s3.blinkdisk.com/upload/" + req.Key
			case msgTypeGetBlob:
				if req.Key == "notfound" {
				} else {
					resp.URL = "https://s3.blinkdisk.com/download/" + req.Key
				}
			case msgTypeDeleteBlob:
			case msgTypeListBlobs:
				resp.Blobs = []BlobInfo{
					{
						Key:      "blob1",
						Size:     100,
						Modified: "2023-01-01T00:00:00Z",
					},
					{
						Key:      "blob2",
						Size:     200,
						Modified: "2023-01-02T00:00:00Z",
					},
				}
			case msgTypeGetMetadata:
				if req.Key == "notfound" {
				} else {
					resp.Size = 100
					resp.Modified = "2023-01-01T00:00:00Z"
				}
			}

			if err := conn.WriteJSON(resp); err != nil {
				t.Errorf("Failed to send response: %v", err)
				return
			}
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

	t.Run("PutBlob", func(t *testing.T) {
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "PUT" {
				w.WriteHeader(http.StatusOK)
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

				if req.Type == msgTypePutBlob {
					resp.URL = httpServer.URL + "/upload/" + req.Key
				}

				conn.WriteJSON(resp)
			}
		}))
		defer server.Close()

		opts.URL = server.URL
		storage, err := New(context.Background(), opts, true)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer storage.Close(context.Background())

		data := gather.FromSlice([]byte("test data"))
		err = storage.PutBlob(context.Background(), "test-blob", data, blob.PutOptions{})
		if err != nil {
			t.Errorf("PutBlob failed: %v", err)
		}
	})

	t.Run("GetBlob", func(t *testing.T) {
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

		opts.URL = server.URL
		storage, err := New(context.Background(), opts, true)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer storage.Close(context.Background())

		output := &gather.WriteBuffer{}
		err = storage.GetBlob(context.Background(), "test-blob", 0, -1, output)
		if err != nil {
			t.Errorf("GetBlob failed: %v", err)
		}

		if string(output.ToByteSlice()) != "test data" {
			t.Errorf("Expected 'test data', got '%s'", string(output.ToByteSlice()))
		}
	})

	t.Run("GetBlobNotFound", func(t *testing.T) {
		output := &gather.WriteBuffer{}
		err := storage.GetBlob(context.Background(), "notfound", 0, -1, output)
		if !errors.Is(err, blob.ErrBlobNotFound) {
			t.Errorf("Expected ErrBlobNotFound, got %v", err)
		}
	})

	t.Run("GetMetadata", func(t *testing.T) {
		metadata, err := storage.GetMetadata(context.Background(), "test-blob")
		if err != nil {
			t.Errorf("GetMetadata failed: %v", err)
		}

		if metadata.Length != 100 {
			t.Errorf("Expected length 100, got %d", metadata.Length)
		}
	})

	t.Run("GetMetadataNotFound", func(t *testing.T) {
		_, err := storage.GetMetadata(context.Background(), "notfound")
		if !errors.Is(err, blob.ErrBlobNotFound) {
			t.Errorf("Expected ErrBlobNotFound, got %v", err)
		}
	})

	t.Run("DeleteBlob", func(t *testing.T) {
		err := storage.DeleteBlob(context.Background(), "test-blob")
		if err != nil {
			t.Errorf("DeleteBlob failed: %v", err)
		}
	})

	t.Run("ListBlobs", func(t *testing.T) {
		var blobs []blob.Metadata
		err := storage.ListBlobs(context.Background(), "", func(bm blob.Metadata) error {
			blobs = append(blobs, bm)
			return nil
		})
		if err != nil {
			t.Errorf("ListBlobs failed: %v", err)
		}

		if len(blobs) != 2 {
			t.Errorf("Expected 2 blobs, got %d", len(blobs))
		}
	})

	t.Run("ConnectionInfo", func(t *testing.T) {
		info := storage.ConnectionInfo()
		if info.Type != bdcStorageType {
			t.Errorf("Expected type %s, got %s", bdcStorageType, info.Type)
		}
	})

	t.Run("DisplayName", func(t *testing.T) {
		name := storage.DisplayName()
		if !strings.Contains(name, "BlinkDisk Cloud") {
			t.Errorf("DisplayName should contain 'BlinkDisk Cloud', got %s", name)
		}
	})
}

func TestBdcStorageRangeRequests(t *testing.T) {
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

		for {
			var req Request
			err := conn.ReadJSON(&req)
			if err != nil {
				return
			}

			var resp Response
			resp.ResponseID = req.RequestID

			if req.Type == msgTypeGetBlob {
				resp.URL = "https://s3.blinkdisk.com/download/" + req.Key
			}

			conn.WriteJSON(resp)
		}
	}))
	defer server.Close()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download/test-blob" {
			rangeHeader := r.Header.Get("Range")
			if rangeHeader != "" {
				w.Header().Set("Content-Range", "bytes 0-4/10")
				w.WriteHeader(http.StatusPartialContent)
				w.Write([]byte("test "))
			} else {
				w.Write([]byte("test data"))
			}
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

	t.Run("RangeRequest", func(t *testing.T) {
		output := &gather.WriteBuffer{}
		err := storage.GetBlob(context.Background(), "test-blob", 0, 5, output)
		if err != nil {
			t.Errorf("GetBlob with range failed: %v", err)
		}

		if string(output.ToByteSlice()) != "test " {
			t.Errorf("Expected 'test ', got '%s'", string(output.ToByteSlice()))
		}
	})

	t.Run("InvalidRange", func(t *testing.T) {
		output := &gather.WriteBuffer{}
		err := storage.GetBlob(context.Background(), "test-blob", -1, 5, output)
		if !errors.Is(err, blob.ErrInvalidRange) {
			t.Errorf("Expected ErrInvalidRange, got %v", err)
		}
	})
}

func TestBdcStorageInterface(t *testing.T) {
	opts := &Options{
		URL:   "ws://localhost:8080",
		Token: "test-token",
	}

	storage, err := New(context.Background(), opts, true)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close(context.Background())

	var _ blob.Storage = storage
	var _ blob.Reader = storage
	var _ blob.Volume = storage
}
