package bdc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	if strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "write: broken pipe") {
		return true
	}

	if strings.Contains(errStr, "connection refused") {
		return true
	}

	if strings.Contains(errStr, "connection reset") {
		return true
	}

	if strings.Contains(errStr, "network is unreachable") {
		return true
	}

	if strings.Contains(errStr, "timeout") {
		return true
	}

	if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
		return true
	}

	if netErr, ok := err.(*net.OpError); ok {
		if netErr.Err == syscall.EPIPE || netErr.Err == syscall.ECONNRESET || netErr.Err == syscall.ECONNREFUSED {
			return true
		}
	}

	return false
}

func (s *bdcStorage) closeConnection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	s.responseMu.Lock()
	for _, ch := range s.responseChans {
		select {
		case ch <- nil:
		default:
		}
	}
	s.responseChans = make(map[string]chan *Response)
	s.responseMu.Unlock()
}

func (s *bdcStorage) connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil && !s.closed {
		return nil
	}

	u, err := url.Parse(s.URL)
	if err != nil {
		return errors.Wrap(err, "invalid URL")
	}

	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	} else if u.Scheme == "" {
		u.Scheme = "ws"
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+s.Token)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return errors.Wrap(err, "failed to connect to BlinkDisk Cloud")
	}

	s.conn = conn
	s.responseChans = make(map[string]chan *Response)
	s.responseReaderDone = make(chan struct{})

	go s.responseReader()

	return nil
}

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
			if isConnectionError(err) {
				s.closeConnection()
			}

			s.responseMu.Lock()
			for _, ch := range s.responseChans {
				select {
				case ch <- nil:
				default:
				}
			}
			s.responseMu.Unlock()
			return
		}

		if resp.Space != nil {
			spaceJSON, err := json.Marshal(resp.Space)
			if err == nil {
				fmt.Fprintln(os.Stderr, "BDC SPACE UPDATE:", string(spaceJSON))
			}
		}

		// TODO: Remove this once the Vault v2 migration is complete
		if resp.Error == "STORAGE_DELETED" {
			fmt.Fprintln(os.Stderr, "BDC STORAGE DELETED: ")
		}

		if resp.Error == "VAULT_DELETED" {
			fmt.Fprintln(os.Stderr, "BDC VAULT DELETED: ")
		}

		s.responseMu.Lock()
		ch, exists := s.responseChans[resp.ResponseID]
		if exists {
			select {
			case ch <- &resp:
			default:
			}
		}
		s.responseMu.Unlock()
	}
}

func (s *bdcStorage) sendRequest(ctx context.Context, req Request) (*Response, error) {
	if err := s.connect(ctx); err != nil {
		return nil, err
	}

	responseCh := make(chan *Response, 1)

	s.responseMu.Lock()
	s.responseChans[req.RequestID] = responseCh
	s.responseMu.Unlock()

	defer func() {
		s.responseMu.Lock()
		delete(s.responseChans, req.RequestID)
		close(responseCh)
		s.responseMu.Unlock()
	}()

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()

		if conn == nil {
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

		s.writeMu.Lock()
		err := conn.WriteJSON(req)
		s.writeMu.Unlock()

		if err != nil {
			if isConnectionError(err) {
				s.closeConnection()

				if attempt < maxRetries-1 {
					time.Sleep(time.Duration(attempt+1) * time.Second)
					continue
				}
			}
			return nil, errors.Wrap(err, "failed to send request")
		}

		select {
		case resp := <-responseCh:
			if resp == nil {
				if attempt < maxRetries-1 {
					time.Sleep(time.Duration(attempt+1) * time.Second)
					continue
				}
				return nil, errors.New("connection closed")
			}
			if resp.Error != "" {
				return nil, errors.New(resp.Error)
			}
			return resp, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
			return nil, errors.New("request timeout")
		}
	}

	return nil, errors.New("max retries exceeded")
}
