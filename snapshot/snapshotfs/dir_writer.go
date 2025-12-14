package snapshotfs

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/compression"
	"github.com/blinkdisk/core/repo/object"
	"github.com/blinkdisk/core/snapshot"
)

// WriteDirManifest writes a directory manifest to the repository and returns the object ID.
func WriteDirManifest(ctx context.Context, rep repo.RepositoryWriter, dirRelativePath string, dirManifest *snapshot.DirManifest, metadataComp compression.Name) (object.ID, error) {
	writer := rep.NewObjectWriter(ctx, object.WriterOptions{
		Description:        "DIR:" + dirRelativePath,
		Prefix:             objectIDPrefixDirectory,
		Compressor:         metadataComp,
		MetadataCompressor: metadataComp,
	})

	defer writer.Close() //nolint:errcheck

	if err := json.NewEncoder(writer).Encode(dirManifest); err != nil {
		return object.EmptyID, errors.Wrap(err, "unable to encode directory JSON")
	}

	oid, err := writer.Result()
	if err != nil {
		return object.EmptyID, errors.Wrap(err, "unable to write directory")
	}

	return oid, nil
}
