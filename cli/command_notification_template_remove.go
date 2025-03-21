package cli

import (
	"context"

	"github.com/blinkdisk/core/notification/notifytemplate"
	"github.com/blinkdisk/core/repo"
)

type commandNotificationTemplateRemove struct {
	notificationTemplateNameArg
}

func (c *commandNotificationTemplateRemove) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("remove", "Remove the notification template").Alias("rm").Alias("delete")
	c.notificationTemplateNameArg.setup(svc, cmd)
	cmd.Action(svc.repositoryWriterAction(c.run))
}

func (c *commandNotificationTemplateRemove) run(ctx context.Context, rep repo.RepositoryWriter) error {
	//nolint:wrapcheck
	return notifytemplate.ResetTemplate(ctx, rep, c.templateName)
}
