package cli

import (
	"context"

	"github.com/blinkdisk/core/internal/apiclient"
	"github.com/blinkdisk/core/internal/serverapi"
)

type commandServerFlush struct {
	sf serverClientFlags
}

func (c *commandServerFlush) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("flush", "Flush the state of BlinkDisk server to persistent storage, etc.")
	c.sf.setup(svc, cmd)
	cmd.Action(svc.serverAction(&c.sf, c.run))
}

func (c *commandServerFlush) run(ctx context.Context, cli *apiclient.BlinkDiskAPIClient) error {
	//nolint:wrapcheck
	return cli.Post(ctx, "control/flush", &serverapi.Empty{}, &serverapi.Empty{})
}
