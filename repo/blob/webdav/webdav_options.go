package webdav

import (
	"github.com/blinkdisk/core/repo/blob/sharded"
	"github.com/blinkdisk/core/repo/blob/throttling"
)

// Options defines options for Filesystem-backed storage.
type Options struct {
	URL                                 string `json:"url"`
	Username                            string `json:"username,omitempty"`
	Password                            string `json:"password,omitempty"                            blinkdisk:"sensitive"`
	TrustedServerCertificateFingerprint string `json:"trustedServerCertificateFingerprint,omitempty"`
	AtomicWrites                        bool   `json:"atomicWrites"`

	sharded.Options
	throttling.Limits
}
