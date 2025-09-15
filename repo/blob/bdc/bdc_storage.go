// Package bdc implements Storage based on BlinkDisk Cloud service.
package bdc

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/blinkdisk/core/internal/iocopy"
	"github.com/blinkdisk/core/repo/blob"
	"github.com/blinkdisk/core/repo/blob/retrying"
)

const (
	bdcStorageType = "bdc"
)

// WebSocket message types
const (
	msgTypePutBlob     = "PUT_BLOB"
	msgTypeGetBlob     = "GET_BLOB"
	msgTypeDeleteBlob  = "DELETE_BLOB"
	msgTypeListBlobs   = "LIST_BLOBS"
	msgTypeGetMetadata = "GET_METADATA"
)

// Request represents a WebSocket request message
type Request struct {
	RequestID string `json:"requestId"`
	Type      string `json:"type"`
	Key       string `json:"key,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Marker    string `json:"marker,omitempty"`
}

// Response represents a WebSocket response message
type Response struct {
	ResponseID string     `json:"responseId"`
	URL        string     `json:"url,omitempty"`
	NextMarker string     `json:"nextMarker,omitempty"`
	Blobs      []BlobInfo `json:"blobs,omitempty"`
	Size       int64      `json:"size,omitempty"`
	Modified   string     `json:"modified,omitempty"`
	Error      string     `json:"error,omitempty"`
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

	conn      *websocket.Conn
	mu        sync.RWMutex
	writeMu   sync.Mutex // Separate mutex for websocket write operations
	closed    bool
	
	// Response handling for parallel requests
	responseChans map[string]chan *Response
	responseMu    sync.RWMutex
	responseReaderDone chan struct{}
}

// generateRequestID generates a random request ID
func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// isConnectionError checks if the error is related to connection issues that should trigger reconnection
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Check for broken pipe errors
	if strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "write: broken pipe") {
		return true
	}
	
	// Check for connection refused errors
	if strings.Contains(errStr, "connection refused") {
		return true
	}
	
	// Check for connection reset errors
	if strings.Contains(errStr, "connection reset") {
		return true
	}
	
	// Check for network unreachable errors
	if strings.Contains(errStr, "network is unreachable") {
		return true
	}
	
	// Check for timeout errors
	if strings.Contains(errStr, "timeout") {
		return true
	}
	
	// Check for WebSocket close errors
	if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
		return true
	}
	
	// Check for underlying network errors
	if netErr, ok := err.(*net.OpError); ok {
		if netErr.Err == syscall.EPIPE || netErr.Err == syscall.ECONNRESET || netErr.Err == syscall.ECONNREFUSED {
			return true
		}
	}
	
	return false
}

// closeConnection closes the current WebSocket connection and cleans up state
func (s *bdcStorage) closeConnection() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	
	// Clean up response channels
	s.responseMu.Lock()
	for _, ch := range s.responseChans {
		select {
		case ch <- nil: // Signal connection closed
		default:
		}
	}
	s.responseChans = make(map[string]chan *Response)
	s.responseMu.Unlock()
}

// connect establishes WebSocket connection to BlinkDisk Cloud
func (s *bdcStorage) connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil && !s.closed {
		return nil
	}

	// Parse URL and convert to WebSocket URL
	u, err := url.Parse(s.URL)
	if err != nil {
		return errors.Wrap(err, "invalid URL")
	}

	// Convert http/https to ws/wss
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	} else if u.Scheme == "" {
		u.Scheme = "ws"
	}

	// Set up headers with authentication
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+s.Token)

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return errors.Wrap(err, "failed to connect to BlinkDisk Cloud")
	}

	s.conn = conn
	
	// Initialize response handling
	s.responseChans = make(map[string]chan *Response)
	s.responseReaderDone = make(chan struct{})
	
	// Start response reader goroutine
	go s.responseReader()
	
	return nil
}

// responseReader reads responses from the WebSocket and routes them to the appropriate channel
func (s *bdcStorage) responseReader() {
	defer close(s.responseReaderDone)
	
	for {
		s.mu.RLock()
		conn := s.conn
		closed := s.closed
		s.mu.RUnlock()
		
		if conn == nil || closed {
			return
		}
		
		var resp Response
		if err := conn.ReadJSON(&resp); err != nil {
			// Connection closed or error occurred
			if isConnectionError(err) {
				// Close connection and signal reconnection needed
				s.closeConnection()
			}
			
			s.responseMu.Lock()
			// Send error to all pending channels
			for _, ch := range s.responseChans {
				select {
				case ch <- nil:
				default:
				}
			}
			s.responseMu.Unlock()
			return
		}
		
		// Route response to the appropriate channel
		s.responseMu.Lock()
		ch, exists := s.responseChans[resp.ResponseID]
		if exists {
			select {
			case ch <- &resp:
			default:
				// Channel is full or closed, skip this response
			}
		}
		s.responseMu.Unlock()
	}
}

// sendRequest sends a request and waits for response
func (s *bdcStorage) sendRequest(ctx context.Context, req Request) (*Response, error) {
	// Try to connect first
	if err := s.connect(ctx); err != nil {
		return nil, err
	}

	// Create response channel for this request
	responseCh := make(chan *Response, 1)
	
	// Register the channel
	s.responseMu.Lock()
	s.responseChans[req.RequestID] = responseCh
	s.responseMu.Unlock()
	
	// Clean up channel when done
	defer func() {
		s.responseMu.Lock()
		delete(s.responseChans, req.RequestID)
		close(responseCh)
		s.responseMu.Unlock()
	}()

	// Retry logic for connection errors
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
			// Try to reconnect
			if err := s.connect(ctx); err != nil {
				return nil, err
			}
			s.mu.RLock()
			conn = s.conn
			s.mu.RUnlock()
		}

		if conn == nil {
			return nil, errors.New("not connected")
		}

		// Use write mutex to ensure only one goroutine can write to websocket at a time
		s.writeMu.Lock()
		err := conn.WriteJSON(req)
		s.writeMu.Unlock()
		
		if err != nil {
			if isConnectionError(err) {
				// Connection error detected, close connection and retry
				s.closeConnection()
				
				if attempt < maxRetries-1 {
					// Wait a bit before retrying
					time.Sleep(time.Duration(attempt+1) * time.Second)
					continue
				}
			}
			return nil, errors.Wrap(err, "failed to send request")
		}

		// Wait for response with timeout
		select {
		case resp := <-responseCh:
			if resp == nil {
				// Connection was closed during request
				if attempt < maxRetries-1 {
					// Wait a bit before retrying
					time.Sleep(time.Duration(attempt+1) * time.Second)
					continue
				}
				return nil, errors.New("connection closed")
			}
			// Check if the response contains an error
			if resp.Error != "" {
				return nil, errors.New(resp.Error)
			}
			return resp, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second): // 30 second timeout
			return nil, errors.New("request timeout")
		}
	}

	return nil, errors.New("max retries exceeded")
}

func (s *bdcStorage) GetBlob(ctx context.Context, id blob.ID, offset, length int64, output blob.OutputBuffer) error {
	output.Reset()

	if offset < 0 {
		return blob.ErrInvalidRange
	}

	req := Request{
		RequestID: generateRequestID(),
		Type:      msgTypeGetBlob,
		Key:       string(id),
	}

	resp, err := s.sendRequest(ctx, req)
	if err != nil {
		return translateError(err)
	}

	if resp.URL == "" {
		return blob.ErrBlobNotFound
	}

	// Download from signed URL
	httpReq, err := http.NewRequestWithContext(ctx, "GET", resp.URL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}

	// Set range header if needed
	if length > 0 {
		httpReq.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	} else if offset > 0 {
		httpReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return errors.Wrap(err, "failed to download blob")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		return blob.ErrBlobNotFound
	}

	if httpResp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return blob.ErrInvalidRange
	}

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusPartialContent {
		return errors.Errorf("unexpected HTTP status: %d", httpResp.StatusCode)
	}

	if length == 0 {
		return nil
	}

	if err := iocopy.JustCopy(output, httpResp.Body); err != nil {
		return errors.Wrap(err, "failed to copy data")
	}

	//nolint:wrapcheck
	return blob.EnsureLengthExactly(output.Length(), length)
}

func (s *bdcStorage) GetMetadata(ctx context.Context, id blob.ID) (blob.Metadata, error) {
	req := Request{
		RequestID: generateRequestID(),
		Type:      msgTypeGetMetadata,
		Key:       string(id),
	}

	resp, err := s.sendRequest(ctx, req)
	if err != nil {
		return blob.Metadata{}, translateError(err)
	}

	if resp.Size == 0 && resp.Modified == "" {
		return blob.Metadata{}, blob.ErrBlobNotFound
	}

	modified, err := time.Parse(time.RFC3339, resp.Modified)
	if err != nil {
		modified = time.Now()
	}

	return blob.Metadata{
		BlobID:    id,
		Length:    resp.Size,
		Timestamp: modified,
	}, nil
}

func (s *bdcStorage) PutBlob(ctx context.Context, id blob.ID, data blob.Bytes, opts blob.PutOptions) error {
	switch {
	case opts.HasRetentionOptions():
		return errors.Wrap(blob.ErrUnsupportedPutBlobOption, "blob-retention")
	case opts.DoNotRecreate:
		return errors.Wrap(blob.ErrUnsupportedPutBlobOption, "do-not-recreate")
	}

	req := Request{
		RequestID: generateRequestID(),
		Type:      msgTypePutBlob,
		Key:       string(id),
		Size:      int64(data.Length()),
	}

	resp, err := s.sendRequest(ctx, req)
	if err != nil {
		return translateError(err)
	}

	if resp.URL == "" {
		return errors.Errorf("no upload URL received, got %v", resp)
	}

	// Upload to signed URL
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", resp.URL, data.Reader())
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}

	httpReq.ContentLength = int64(data.Length())

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return errors.Wrap(err, "failed to upload blob")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected HTTP status: %d", httpResp.StatusCode)
	}

	if opts.GetModTime != nil {
		*opts.GetModTime = time.Now()
	}

	return nil
}

func (s *bdcStorage) DeleteBlob(ctx context.Context, id blob.ID) error {
	req := Request{
		RequestID: generateRequestID(),
		Type:      msgTypeDeleteBlob,
		Key:       string(id),
	}

	_, err := s.sendRequest(ctx, req)
	if err != nil {
		return translateError(err)
	}

	return nil
}

func (s *bdcStorage) ListBlobs(ctx context.Context, prefix blob.ID, callback func(blob.Metadata) error) error {
	marker := ""

	for {
		req := Request{
			RequestID: generateRequestID(),
			Type:      msgTypeListBlobs,
			Prefix:    string(prefix),
			Marker:    marker,
		}

		resp, err := s.sendRequest(ctx, req)
		if err != nil {
			return translateError(err)
		}

		for _, blobInfo := range resp.Blobs {
			modified, err := time.Parse(time.RFC3339, blobInfo.Modified)
			if err != nil {
				modified = time.Now()
			}

			bm := blob.Metadata{
				BlobID:    blob.ID(blobInfo.Key),
				Length:    blobInfo.Size,
				Timestamp: modified,
			}

			if err := callback(bm); err != nil {
				return err
			}
		}

		marker = resp.NextMarker
		if marker == "" {
			break
		}
	}

	return nil
}

func (s *bdcStorage) ConnectionInfo() blob.ConnectionInfo {
	return blob.ConnectionInfo{
		Type:   bdcStorageType,
		Config: &s.Options,
	}
}

func (s *bdcStorage) DisplayName() string {
	return fmt.Sprintf("BlinkDisk Cloud: %v", s.URL)
}

func (s *bdcStorage) String() string {
	return fmt.Sprintf("bdc://%s", s.URL)
}

func (s *bdcStorage) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil && !s.closed {
		s.closed = true
		
		// Close the connection to stop the response reader
		err := s.conn.Close()
		
		// Wait for response reader to finish
		select {
		case <-s.responseReaderDone:
		case <-ctx.Done():
			return ctx.Err()
		}
		
		// Clean up any remaining response channels
		s.responseMu.Lock()
		for _, ch := range s.responseChans {
			close(ch)
		}
		s.responseChans = make(map[string]chan *Response)
		s.responseMu.Unlock()
		
		return err
	}

	return nil
}

func (s *bdcStorage) FlushCaches(ctx context.Context) error {
	return nil
}

func (s *bdcStorage) ExtendBlobRetention(ctx context.Context, id blob.ID, opts blob.ExtendOptions) error {
	return errors.Wrap(blob.ErrUnsupportedPutBlobOption, "blob-retention")
}

func (s *bdcStorage) IsReadOnly() bool {
	return false
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	// Check for connection errors that should trigger reconnection
	if isConnectionError(err) {
		return errors.Wrap(err, "connection error")
	}

	if strings.Contains(err.Error(), "authentication") {
		return blob.ErrInvalidCredentials
	}

	// Check for HTTP errors
	if strings.Contains(err.Error(), "404") {
		return blob.ErrBlobNotFound
	}

	if strings.Contains(err.Error(), "416") {
		return blob.ErrInvalidRange
	}

	return err
}

// New creates new BlinkDisk Cloud-backed storage with specified options.
func New(ctx context.Context, opt *Options, isCreate bool) (blob.Storage, error) {
	_ = isCreate

	if opt.URL == "" {
		return nil, errors.New("URL must be specified")
	}

	if opt.Token == "" {
		return nil, errors.New("token must be specified")
	}

	// Validate URL format
	u, err := url.Parse(opt.URL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid URL")
	}

	if u.Scheme == "" {
		return nil, errors.New("URL must include scheme (http://, https://, ws://, or wss://)")
	}

	storage := &bdcStorage{
		Options: *opt,
	}

	return retrying.NewWrapper(storage), nil
}

func init() {
	blob.AddSupportedStorage(bdcStorageType, Options{}, New)
}
