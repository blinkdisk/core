package format

import (
	"context"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/internal/feature"
	"github.com/blinkdisk/core/repo/blob"
)

// SetParameters sets the mutable repository parameters.
func (m *Manager) SetParameters(
	ctx context.Context,
	mp MutableParameters,
	blobcfg BlobStorageConfiguration,
	requiredFeatures []feature.Required,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := mp.Validate(); err != nil {
		return errors.Wrap(err, "invalid parameters")
	}

	if err := blobcfg.Validate(); err != nil {
		return errors.Wrap(err, "invalid blob-config options")
	}

	m.repoConfig.MutableParameters = mp
	m.repoConfig.RequiredFeatures = requiredFeatures

	if err := m.j.EncryptRepositoryConfig(m.repoConfig, m.formatEncryptionKey); err != nil {
		return errors.New("unable to encrypt format bytes")
	}

	if err := m.j.WriteBlobCfgBlob(ctx, m.blobs, blobcfg, m.formatEncryptionKey); err != nil {
		return errors.Wrap(err, "unable to write blobcfg blob")
	}

	// At this point the new blobcfg is persisted in the blob layer. Setting this
	// here also ensures the call below properly sets retention on the blinkdisk
	// repository blob.
	m.blobCfgBlob = blobcfg

	if err := m.j.WriteBlinkDiskRepositoryBlob(ctx, m.blobs, m.blobCfgBlob); err != nil {
		return errors.Wrap(err, "unable to write format blob")
	}

	m.cache.Remove(ctx, []blob.ID{BlinkDiskRepositoryBlobID, BlinkDiskBlobCfgBlobID})

	return nil
}
