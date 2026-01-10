package repo

import (
	"github.com/blinkdisk/core/internal/epoch"
	"github.com/blinkdisk/core/repo/blob"
	"github.com/blinkdisk/core/repo/content"
	"github.com/blinkdisk/core/repo/content/indexblob"
	"github.com/blinkdisk/core/repo/format"
)

// GetLockingStoragePrefixes Return all prefixes that may be maintained by Object Locking.
func GetLockingStoragePrefixes() []blob.ID {
	return append([]blob.ID{
		blob.ID(indexblob.V0IndexBlobPrefix),
		blob.ID(epoch.EpochManagerIndexUberPrefix),
		blob.ID(format.BlinkDiskRepositoryBlobID),
		blob.ID(format.BlinkDiskBlobCfgBlobID),
	}, content.PackBlobIDPrefixes...)
}
