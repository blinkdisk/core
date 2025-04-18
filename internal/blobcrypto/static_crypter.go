package blobcrypto

import (
	"github.com/blinkdisk/core/repo/encryption"
	"github.com/blinkdisk/core/repo/hashing"
)

// StaticCrypter implements Crypter interface with static hash and encryption functions.
type StaticCrypter struct {
	Hash       hashing.HashFunc
	Encryption encryption.Encryptor
}

// Encryptor returns the encryption algorithm.
func (p StaticCrypter) Encryptor() encryption.Encryptor {
	return p.Encryption
}

// HashFunc returns the hashing algorithm.
func (p StaticCrypter) HashFunc() hashing.HashFunc {
	return p.Hash
}

var _ Crypter = (*StaticCrypter)(nil)
