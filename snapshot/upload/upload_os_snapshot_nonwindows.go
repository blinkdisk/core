//go:build !windows

package upload

import (
	"context"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/fs"
	"github.com/blinkdisk/core/snapshot/policy"
)

func osSnapshotMode(*policy.OSSnapshotPolicy) policy.OSSnapshotMode {
	return policy.OSSnapshotNever
}

func createOSSnapshot(context.Context, fs.Directory, *policy.OSSnapshotPolicy) (newRoot fs.Directory, cleanup func(), err error) {
	return nil, nil, errors.New("not supported on this platform")
}
