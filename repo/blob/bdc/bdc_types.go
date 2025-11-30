package bdc

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kopia/kopia/repo/blob"
)

// Request represents a WebSocket request message
type Request struct {
	RequestID string `json:"requestId"`
	Type      string `json:"type"`
	Key       string `json:"key,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	Length    int64  `json:"length,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Marker    string `json:"marker,omitempty"`
}

// SpaceStats represents space information in responses
type SpaceStats struct {
	Capacity int64 `json:"capacity"`
	Used     int64 `json:"used"`
}

// Response represents a WebSocket response message
type Response struct {
	ResponseID string      `json:"responseId"`
	URL        string      `json:"url,omitempty"`
	NextMarker string      `json:"nextMarker,omitempty"`
	Blobs      []BlobInfo  `json:"blobs,omitempty"`
	Size       int64       `json:"size,omitempty"`
	Modified   string      `json:"modified,omitempty"`
	Error      string      `json:"error,omitempty"`
	Space      *SpaceStats `json:"space,omitempty"`
}

// BlobInfo represents blob information in list responses
type BlobInfo struct {
	Key      string `json:"key"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

type bdcStorage struct {
	Options
	blob.DefaultProviderImplementation

	conn    *websocket.Conn
	mu      sync.RWMutex
	writeMu sync.Mutex
	closed  bool

	responseChans      map[string]chan *Response
	responseMu         sync.RWMutex
	responseReaderDone chan struct{}
}
