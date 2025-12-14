package maintenance

import (
	"context"

	"github.com/blinkdisk/core/internal/contentlog"
	"github.com/blinkdisk/core/repo/content/indexblob"
	"github.com/blinkdisk/core/repo/maintenancestats"
)

// runTaskIndexCompactionQuick rewrites index blobs to reduce their count but does not drop any contents.
func runTaskIndexCompactionQuick(ctx context.Context, runParams RunParameters, s *Schedule, safety SafetyParameters) error {
	return reportRunAndMaybeCheckContentIndex(ctx, runParams.rep, TaskIndexCompaction, s, func() (maintenancestats.Kind, error) {
		log := runParams.rep.LogManager().NewLogger("maintenance-index-compaction")

		contentlog.Log(ctx, log, "Compacting indexes...")

		const maxSmallBlobsForIndexCompaction = 8

		return runParams.rep.ContentManager().CompactIndexes(ctx, indexblob.CompactOptions{
			MaxSmallBlobs:                    maxSmallBlobsForIndexCompaction,
			DisableEventualConsistencySafety: safety.DisableEventualConsistencySafety,
		})
	})
}
