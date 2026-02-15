package maintenance

import (
	"context"
	"time"

	"github.com/blinkdisk/core/internal/contentlog"
	"github.com/blinkdisk/core/internal/contentlog/logparam"
	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/content/indexblob"
	"github.com/blinkdisk/core/repo/maintenancestats"
)

// dropDeletedContents rewrites indexes while dropping deleted contents above certain age.
func dropDeletedContents(ctx context.Context, rep repo.DirectRepositoryWriter, dropDeletedBefore time.Time, safety SafetyParameters) (*maintenancestats.CompactIndexesStats, error) {
	ctx = contentlog.WithParams(ctx,
		logparam.String("span:drop-deleted-contents", contentlog.RandomSpanID()))

	log := rep.LogManager().NewLogger("maintenance-drop-deleted-contents")

	contentlog.Log1(ctx, log, "Dropping deleted contents", logparam.Time("dropDeletedBefore", dropDeletedBefore))

	//nolint:wrapcheck
	return rep.ContentManager().CompactIndexes(ctx, indexblob.CompactOptions{
		AllIndexes:                       true,
		DropDeletedBefore:                dropDeletedBefore,
		DisableEventualConsistencySafety: safety.DisableEventualConsistencySafety,
	})
}
