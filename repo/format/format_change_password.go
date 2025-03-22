package format

import (
	"context"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/repo/blob"
)

// ChangePassword changes the repository password and rewrites
// `blinkdisk.repository` & `blinkdisk.blobcfg`.
func (m *Manager) ChangePassword(ctx context.Context, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.repoConfig.EnablePasswordChange {
		return errors.New("password changes are not supported for repositories created using BlinkDisk v0.8 or older")
	}

	newFormatEncryptionKey, err := m.j.DeriveFormatEncryptionKeyFromPassword(newPassword)
	if err != nil {
		return errors.Wrap(err, "unable to derive master key")
	}

	m.formatEncryptionKey = newFormatEncryptionKey
	m.password = newPassword

	if err := m.j.EncryptRepositoryConfig(m.repoConfig, newFormatEncryptionKey); err != nil {
		return errors.Wrap(err, "unable to encrypt format bytes")
	}

	if err := m.j.WriteBlobCfgBlob(ctx, m.blobs, m.blobCfgBlob, newFormatEncryptionKey); err != nil {
		return errors.Wrap(err, "unable to write blobcfg blob")
	}

	if err := m.j.WriteBlinkDiskRepositoryBlob(ctx, m.blobs, m.blobCfgBlob); err != nil {
		return errors.Wrap(err, "unable to write format blob")
	}

	m.cache.Remove(ctx, []blob.ID{BlinkDiskRepositoryBlobID, BlinkDiskBlobCfgBlobID})

	return nil
}
