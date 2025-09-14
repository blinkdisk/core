package cli

import (
	"context"

	"github.com/alecthomas/kingpin/v2"

	"github.com/blinkdisk/core/repo/blob"
	"github.com/blinkdisk/core/repo/blob/bdc"
)

type storageBdcFlags struct {
	bdcOptions bdc.Options
}

func (c *storageBdcFlags) Setup(svc StorageProviderServices, cmd *kingpin.CmdClause) {
	cmd.Flag("url", "URL of the BlinkDisk Cloud API server").Required().StringVar(&c.bdcOptions.URL)
	cmd.Flag("token", "BlinkDisk Cloud access token").Required().Envar(svc.EnvName("BLINKDISK_CLOUD_TOKEN")).StringVar(&c.bdcOptions.Token)

	commonThrottlingFlags(cmd, &c.bdcOptions.Limits)
}

func (c *storageBdcFlags) Connect(ctx context.Context, isCreate bool, formatVersion int) (blob.Storage, error) {
	_ = formatVersion

	//nolint:wrapcheck
	return bdc.New(ctx, &c.bdcOptions, isCreate)
}
