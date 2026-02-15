package bdc

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/repo/blob"
	"github.com/kopia/kopia/repo/blob/retrying"
)

func (s *bdcStorage) ConnectionInfo() blob.ConnectionInfo {
	return blob.ConnectionInfo{
		Type:   bdcStorageType,
		Config: &s.Options,
	}
}

func (s *bdcStorage) DisplayName() string {
	return fmt.Sprintf("CloudBlink: %v", s.URL)
}

func (s *bdcStorage) String() string {
	return fmt.Sprintf("bdc://%s", s.URL)
}

func (s *bdcStorage) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil && !s.closed {
		s.closed = true

		err := s.conn.Close()

		select {
		case <-s.responseReaderDone:
		case <-ctx.Done():
			return ctx.Err()
		}

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

func New(ctx context.Context, opt *Options, isCreate bool) (blob.Storage, error) {
	_ = isCreate

	if opt.URL == "" {
		return nil, errors.New("URL must be specified")
	}

	if opt.Token == "" {
		return nil, errors.New("token must be specified")
	}

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
