package bdc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/internal/iocopy"
	"github.com/kopia/kopia/repo/blob"
)

func (s *bdcStorage) GetBlob(ctx context.Context, id blob.ID, offset, length int64, output blob.OutputBuffer) error {
	output.Reset()

	if offset < 0 {
		return blob.ErrInvalidRange
	}

	req := Request{
		RequestID: generateRequestID(),
		Type:      msgTypeGetBlob,
		Key:       string(id),
		Offset:    offset,
		Length:    length,
	}

	resp, err := s.sendRequest(ctx, req)
	if err != nil {
		return translateError(err)
	}

	if resp.URL == "" {
		return blob.ErrBlobNotFound
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", resp.URL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}

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
