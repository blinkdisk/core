package cli

import (
	"context"

	"github.com/blinkdisk/core/internal/apiclient"
	"github.com/blinkdisk/core/internal/serverapi"
)

type commandServerRefresh struct {
	sf serverClientFlags
}

func (c *commandServerRefresh) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("refresh", "Refresh the cache in BlinkDisk server to observe new sources, etc.")
	c.sf.setup(svc, cmd)
	cmd.Action(svc.serverAction(&c.sf, c.run))
}

func (c *commandServerRefresh) run(ctx context.Context, cli *apiclient.BlinkDiskAPIClient) error {
	//nolint:wrapcheck
	return cli.Post(ctx, "control/refresh", &serverapi.Empty{}, &serverapi.Empty{})
}
