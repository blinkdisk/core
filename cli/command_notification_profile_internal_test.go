package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/internal/repotesting"
	"github.com/blinkdisk/core/notification/notifyprofile"
	"github.com/blinkdisk/core/notification/sender"
)

func TestNotificationProfileAutocomplete(t *testing.T) {
	t.Parallel()

	var a notificationProfileFlag

	ctx, env := repotesting.NewEnvironment(t, repotesting.FormatNotImportant)

	require.Empty(t, a.listNotificationProfiles(ctx, env.Repository))
	require.NoError(t, notifyprofile.SaveProfile(ctx, env.RepositoryWriter, notifyprofile.Config{
		ProfileName: "test-profile",
		MethodConfig: sender.MethodConfig{
			Type:   "email",
			Config: map[string]string{},
		},
	}))
	require.NoError(t, env.RepositoryWriter.Flush(ctx))

	require.Contains(t, a.listNotificationProfiles(ctx, env.Repository), "test-profile")

	a.profileName = "no-such-profile"
	require.Empty(t, a.listNotificationProfiles(ctx, env.Repository))
}
