//go:build !testing
// +build !testing

package format

import "github.com/blinkdisk/core/internal/crypto"

// DefaultKeyDerivationAlgorithm is the derivation algorithm for format encryption for new repositories.
const DefaultKeyDerivationAlgorithm = crypto.ScryptAlgorithm

// SupportedFormatBlobKeyDerivationAlgorithms returns the supported algorithms
// for deriving the local cache encryption key when connecting to a repository
// via the blinkdisk API server.
func SupportedFormatBlobKeyDerivationAlgorithms() []string {
	return []string{crypto.ScryptAlgorithm, crypto.Pbkdf2Algorithm}
}
