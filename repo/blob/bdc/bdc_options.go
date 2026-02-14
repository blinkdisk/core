package bdc

import "github.com/kopia/kopia/repo/blob/throttling"

// Options defines options for BlinkCloud storage.
type Options struct {
	// URL is the host URL for the BlinkCloud service.
	URL string `json:"url"`

	// Token is used for authentication with the BlinkCloud service.
	Token string `json:"token" kopia:"sensitive"`

	// Version is used to specify the BlinkCloud API version.
	Version int `json:"version"`

	throttling.Limits
}
