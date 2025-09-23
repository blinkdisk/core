package bdc

import "github.com/blinkdisk/core/repo/blob/throttling"

// Options defines options for BlinkDisk Cloud storage.
type Options struct {
	// URL is the host URL for the BlinkDisk Cloud service.
	URL string `json:"url"`

	// Token is used for authentication with the BlinkDisk Cloud service.
	Token string `json:"token" blinkdisk:"sensitive"`

	// Version is used to specify the BlinkDisk Cloud API version.
	Version int `json:"version"`

	throttling.Limits
}
