package cli

import (
	"context"

	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/maintenance"
	"github.com/blinkdisk/core/snapshot/snapshotmaintenance"
)

type commandMaintenanceRun struct {
	maintenanceRunFull  bool
	maintenanceRunForce bool
	safety              maintenance.SafetyParameters
}

func (c *commandMaintenanceRun) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("run", "Run repository maintenance")
	cmd.Flag("full", "Full maintenance").BoolVar(&c.maintenanceRunFull)
	cmd.Flag("force", "Run maintenance even if not owned (unsafe)").Hidden().BoolVar(&c.maintenanceRunForce)
	safetyFlagVar(cmd, &c.safety)

	cmd.Action(svc.directRepositoryWriteAction(c.run))
}

func (c *commandMaintenanceRun) run(ctx context.Context, rep repo.DirectRepositoryWriter) error {
	mode := maintenance.ModeQuick

	if c.maintenanceRunFull {
		mode = maintenance.ModeFull
	}

	//nolint:wrapcheck
	return snapshotmaintenance.Run(ctx, rep, mode, c.maintenanceRunForce, c.safety)
}
