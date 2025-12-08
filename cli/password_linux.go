package cli

import (
	"github.com/alecthomas/kingpin/v2"
)

func (c *App) setupOSSpecificKeychainFlags(svc appServices, app *kingpin.Application) {
	app.Flag("use-keyring", "Use Gnome Keyring for storing repository password.").Default("false").Envar(svc.EnvName("BLINKDISK_USE_KEYRING")).BoolVar(&c.keyRingEnabled)
}
