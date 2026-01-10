package tempfile_test

import (
	"testing"

	"github.com/blinkdisk/core/internal/tempfile"
)

func TestTempFile(t *testing.T) {
	tempfile.VerifyTempfile(t, tempfile.CreateAutoDelete)
}
