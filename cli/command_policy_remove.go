package cli

import (
	"context"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/snapshot/policy"
)

type commandPolicyDelete struct {
	policyTargetFlags
	dryRun bool
}

func (c *commandPolicyDelete) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("delete", "Remove snapshot policy for a single directory, user@host or a global policy.").Alias("remove").Alias("rm")
	c.policyTargetFlags.setup(cmd)
	cmd.Flag("dry-run", "Do not remove").Short('n').BoolVar(&c.dryRun)
	cmd.Action(svc.repositoryWriterAction(c.run))
}

func (c *commandPolicyDelete) run(ctx context.Context, rep repo.RepositoryWriter) error {
	targets, err := c.policyTargets(ctx, rep)
	if err != nil {
		return err
	}

	for _, target := range targets {
		log(ctx).Infof("Removing policy on %q...", target)

		if c.dryRun {
			continue
		}

		if err := policy.RemovePolicy(ctx, rep, target); err != nil {
			return errors.Wrapf(err, "error removing policy on %v", target)
		}
	}

	return nil
}
