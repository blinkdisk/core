package bdc

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/repo/blob"
)

func translateError(err error) error {
	if err == nil {
		return nil
	}

	if isConnectionError(err) {
		return errors.Wrap(err, "connection error")
	}

	if strings.Contains(err.Error(), "authentication") {
		return blob.ErrInvalidCredentials
	}

	if strings.Contains(err.Error(), "404") {
		return blob.ErrBlobNotFound
	}

	if strings.Contains(err.Error(), "416") {
		return blob.ErrInvalidRange
	}

	return err
}
