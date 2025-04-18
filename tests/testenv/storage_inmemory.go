package testenv

import (
	"context"

	"github.com/alecthomas/kingpin/v2"

	"github.com/blinkdisk/core/cli"
	"github.com/blinkdisk/core/internal/repotesting"
	"github.com/blinkdisk/core/repo/blob"
)

// storageInMemoryFlags is in-memory storage initialization flags for cli
// setup.
type storageInMemoryFlags struct {
	options repotesting.ReconnectableStorageOptions
}

func (c *storageInMemoryFlags) Setup(_ cli.StorageProviderServices, cmd *kingpin.CmdClause) {
	cmd.Flag("uuid", "UUID of the reconnectable in-memory storage").Required().StringVar(&c.options.UUID)
}

func (c *storageInMemoryFlags) Connect(ctx context.Context, isCreate bool, _ int) (blob.Storage, error) {
	//nolint:wrapcheck
	return blob.NewStorage(ctx, blob.ConnectionInfo{
		Type:   repotesting.ReconnectableStorageType,
		Config: &c.options,
	}, isCreate)
}
