package cli

import (
	"context"

	"github.com/blinkdisk/core/internal/apiclient"
)

type commandServerUpload struct {
	commandServerSourceManagerAction
}

func (c *commandServerUpload) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("snapshot", "Trigger upload for one or more existing sources").Alias("upload")

	c.commandServerSourceManagerAction.setup(svc, cmd)
	cmd.Action(svc.serverAction(&c.sf, c.run))
}

func (c *commandServerUpload) run(ctx context.Context, cli *apiclient.BlinkDiskAPIClient) error {
	return c.triggerActionOnMatchingSources(ctx, cli, "control/trigger-snapshot")
}
