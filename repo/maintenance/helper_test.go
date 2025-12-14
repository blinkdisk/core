package maintenance

import (
	"context"

	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/maintenancestats"
)

// helpers exported for tests

func ExtendBlobRetentionTime(ctx context.Context, rep repo.DirectRepositoryWriter, opt ExtendBlobRetentionTimeOptions) (*maintenancestats.ExtendBlobRetentionStats, error) {
	return extendBlobRetentionTime(ctx, rep, opt)
}
