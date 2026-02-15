package bdc

import "github.com/blinkdisk/core/repo/blob/throttling"

// Options defines options for CloudBlink storage.
type Options struct {
	// URL is the host URL for the CloudBlink service.
	URL string `json:"url"`

	// Token is used for authentication with the CloudBlink service.
	Token string `json:"token" blinkdisk:"sensitive"`

	// Version is used to specify the CloudBlink API version.
	Version int `json:"version"`

	throttling.Limits
}
