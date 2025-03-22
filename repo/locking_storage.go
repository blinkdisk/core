package repo

import (
	"github.com/blinkdisk/core/internal/epoch"
	"github.com/blinkdisk/core/repo/content"
	"github.com/blinkdisk/core/repo/content/indexblob"
	"github.com/blinkdisk/core/repo/format"
)

// GetLockingStoragePrefixes Return all prefixes that may be maintained by Object Locking.
func GetLockingStoragePrefixes() []string {
	var prefixes []string
	// collect prefixes that need to be locked on put
	for _, prefix := range content.PackBlobIDPrefixes {
		prefixes = append(prefixes, string(prefix))
	}

	prefixes = append(prefixes, indexblob.V0IndexBlobPrefix, epoch.EpochManagerIndexUberPrefix, format.BlinkDiskRepositoryBlobID,
		format.BlinkDiskBlobCfgBlobID)

	return prefixes
}
