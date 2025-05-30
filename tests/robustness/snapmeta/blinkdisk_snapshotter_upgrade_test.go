//go:build darwin || (linux && amd64)
// +build darwin linux,amd64

package snapmeta

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/repo/format"
	"github.com/blinkdisk/core/tests/tools/blinkdiskrunner"
)

func TestGetRepositoryStatus(t *testing.T) {
	repoDir := t.TempDir()

	ks, err := NewSnapshotter(repoDir)
	if errors.Is(err, blinkdiskrunner.ErrExeVariableNotSet) {
		t.Skip("BLINKDISK_EXE not set, skipping test")
	}

	err = ks.ConnectOrCreateRepo(repoDir)
	require.NoError(t, err)

	rs, err := ks.GetRepositoryStatus()
	require.NoError(t, err)

	prev := rs.ContentFormat.Version
	require.Equal(t, prev, format.Version(3), "The format version should be 3.")
}
