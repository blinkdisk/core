package snapshotfs

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/fs"
	"github.com/blinkdisk/core/snapshot"
)

const directoryStreamType = "blinkdisk:directory"

// readDirEntries reads all directory entries from the specified reader.
func readDirEntries(r io.Reader) ([]*snapshot.DirEntry, *fs.DirectorySummary, error) {
	var dir snapshot.DirManifest

	if err := json.NewDecoder(r).Decode(&dir); err != nil {
		return nil, nil, errors.Wrap(err, "unable to parse directory object")
	}

	if dir.StreamType != directoryStreamType {
		return nil, nil, errors.New("invalid directory stream type")
	}

	return dir.Entries, dir.Summary, nil
}
