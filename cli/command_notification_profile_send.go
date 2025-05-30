package cli

import (
	"context"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/notification"
	"github.com/blinkdisk/core/notification/notifyprofile"
	"github.com/blinkdisk/core/notification/sender"
	"github.com/blinkdisk/core/repo"
)

type commandNotificationProfileTest struct {
	notificationProfileFlag
}

func (c *commandNotificationProfileTest) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("test", "Send test notification").Alias("send-test-message")

	c.notificationProfileFlag.setup(svc, cmd)

	cmd.Action(svc.repositoryReaderAction(c.run))
}

func (c *commandNotificationProfileTest) run(ctx context.Context, rep repo.Repository) error {
	p, err := notifyprofile.GetProfile(ctx, rep, c.profileName)
	if err != nil {
		return errors.Wrap(err, "unable to get notification profile")
	}

	snd, err := sender.GetSender(ctx, p.ProfileName, p.MethodConfig.Type, p.MethodConfig.Config)
	if err != nil {
		return errors.Wrap(err, "unable to get notification sender")
	}

	return notification.SendTestNotification(ctx, rep, snd) //nolint:wrapcheck
}
