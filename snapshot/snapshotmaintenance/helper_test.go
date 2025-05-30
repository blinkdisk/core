package snapshotmaintenance_test

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/fs"
	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/snapshot"
	"github.com/blinkdisk/core/snapshot/policy"
	"github.com/blinkdisk/core/snapshot/upload"
)

// Create snapshots an FS entry.
func createSnapshot(ctx context.Context, rep repo.RepositoryWriter, e fs.Entry, si snapshot.SourceInfo, description string) (*snapshot.Manifest, error) {
	// sanitize source path
	si.Path = filepath.Clean(si.Path)

	policyTree, err := policy.TreeForSource(ctx, rep, si)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get policy tree")
	}

	previous, err := snapshot.FindPreviousManifests(ctx, rep, si, nil)
	if err != nil {
		return nil, err
	}

	u := upload.NewUploader(rep)

	manifest, err := u.Upload(ctx, e, policyTree, si, previous...)
	if err != nil {
		return nil, err
	}

	manifest.Description = description
	if _, err = snapshot.SaveSnapshot(ctx, rep, manifest); err != nil {
		return nil, errors.Wrap(err, "cannot save manifest")
	}

	return manifest, nil
}
