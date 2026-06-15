package bdc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
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

const (
	maxRequestRetries = 3
	requestTimeout    = 30 * time.Second
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

	if stderrors.Is(err, io.EOF) ||
		stderrors.Is(err, syscall.EPIPE) ||
		stderrors.Is(err, syscall.ECONNRESET) ||
		stderrors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
		return true
	}

	var netErr *net.OpError
	if stderrors.As(err, &netErr) && isConnectionError(netErr.Err) {
		return true
	}

	errStr := strings.ToLower(err.Error())
	for _, marker := range []string{
		"broken pipe",
		"connection refused",
		"connection reset",
		"connection reset by peer",
		"connection aborted",
		"connection closed",
		"forcibly closed by the remote host",
		"use of closed network connection",
		"network is unreachable",
		"no route to host",
		"wsasend",
		"wsarecv",
		"timeout",
	} {
		if strings.Contains(errStr, marker) {
			return true
		}
	}

	return false
}

func (s *bdcStorage) closeConnection(connToClose *websocket.Conn) {
	shouldSignal := false

	s.mu.Lock()

	if s.conn != nil && (connToClose == nil || s.conn == connToClose) {
		_ = s.conn.Close()
		s.conn = nil
		shouldSignal = true
	}

	s.mu.Unlock()

	if !shouldSignal {
		return
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
		return errors.Wrap(err, "failed to connect to CloudBlink")
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
			s.closeConnection(conn)
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

func (s *bdcStorage) registerResponseChannel(requestID string) chan *Response {
	responseCh := make(chan *Response, 1)

	s.responseMu.Lock()
	if s.responseChans == nil {
		s.responseChans = make(map[string]chan *Response)
	}
	s.responseChans[requestID] = responseCh
	s.responseMu.Unlock()

	return responseCh
}

func (s *bdcStorage) unregisterResponseChannel(requestID string, responseCh chan *Response) {
	s.responseMu.Lock()
	if s.responseChans[requestID] == responseCh {
		delete(s.responseChans, requestID)
	}
	close(responseCh)
	s.responseMu.Unlock()
}

func (s *bdcStorage) connection(ctx context.Context) (*websocket.Conn, error) {
	if err := s.connect(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return nil, errors.New("not connected")
	}

	return conn, nil
}

func (s *bdcStorage) sendRequestAttempt(ctx context.Context, req Request) (*Response, error) {
	conn, err := s.connection(ctx)
	if err != nil {
		return nil, err
	}

	responseCh := s.registerResponseChannel(req.RequestID)

	defer func() {
		s.unregisterResponseChannel(req.RequestID, responseCh)
	}()

	s.writeMu.Lock()
	err = conn.WriteJSON(req)
	s.writeMu.Unlock()

	if err != nil {
		if isConnectionError(err) {
			s.closeConnection(conn)
		}

		return nil, errors.Wrap(err, "failed to send request")
	}

	timer := time.NewTimer(requestTimeout)
	defer timer.Stop()

	select {
	case resp := <-responseCh:
		if resp == nil {
			return nil, errors.New("connection closed")
		}
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		s.closeConnection(conn)
		return nil, errors.New("request timeout")
	}
}

func requestRetryDelay(attempt int) time.Duration {
	return time.Duration(attempt+1) * time.Second
}

func sleepBeforeRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(requestRetryDelay(attempt))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *bdcStorage) sendRequest(ctx context.Context, req Request) (*Response, error) {
	var lastErr error

	for attempt := 0; attempt < maxRequestRetries; attempt++ {
		attemptReq := req
		if attemptReq.RequestID == "" || attempt > 0 {
			attemptReq.RequestID = generateRequestID()
		}

		resp, err := s.sendRequestAttempt(ctx, attemptReq)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isConnectionError(err) {
			return nil, err
		}

		if attempt < maxRequestRetries-1 {
			if err := sleepBeforeRetry(ctx, attempt); err != nil {
				return nil, err
			}
		}
	}

	return nil, errors.Wrap(lastErr, "max retries exceeded")
}
