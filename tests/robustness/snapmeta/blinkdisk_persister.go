//go:build darwin || (linux && amd64)

// Package snapmeta provides BlinkDisk implementations of Persister and Snapshotter.
package snapmeta

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blinkdisk/core/tests/robustness"
)

// BlinkDiskPersister implements robustness.Persister.
type BlinkDiskPersister struct {
	*Simple
	localMetadataDir string
	persistenceDir   string
	blinkdiskConnector
}

var _ robustness.Persister = (*BlinkDiskPersister)(nil)

// NewPersister returns a BlinkDisk based Persister.
// ConnectOrCreateRepo must be invoked to enable the interface.
func NewPersister(baseDir string) (*BlinkDiskPersister, error) {
	localDir, err := os.MkdirTemp(baseDir, "blinkdisk-local-metadata-")
	if err != nil {
		return nil, err
	}

	persistenceDir, err := os.MkdirTemp(localDir, "blinkdisk-persistence-root")
	if err != nil {
		return nil, err
	}

	km := &BlinkDiskPersister{
		localMetadataDir: localDir,
		persistenceDir:   persistenceDir,
		Simple:           NewSimple(),
	}

	if err := km.initializeConnector(localDir); err != nil {
		return nil, err
	}

	km.initS3WithServerFn = km.persisterInitS3WithServer
	km.initFilesystemWithServerFn = km.persisterInitFilesystemWithServer

	return km, nil
}

// persisterInitS3WithServer is an adaptor for initS3() as the persister
// does not support the server configuration.
func (store *BlinkDiskPersister) persisterInitS3WithServer(repoPath, bucketName, addr string) error {
	return store.initS3(repoPath, bucketName)
}

// persisterInitFilesystemWithServer is an adaptor for initFilesystem() as the persister
// does not support the server configuration.
func (store *BlinkDiskPersister) persisterInitFilesystemWithServer(repoPath, addr string) error {
	return store.initFilesystem(repoPath)
}

// ConnectOrCreateRepo makes the Persister ready for use.
func (store *BlinkDiskPersister) ConnectOrCreateRepo(repoPath string) error {
	return store.connectOrCreateRepo(repoPath)
}

// Cleanup cleans up the local temporary files used by a BlinkDiskMetadata.
func (store *BlinkDiskPersister) Cleanup() {
	if store.localMetadataDir != "" {
		os.RemoveAll(store.localMetadataDir) //nolint:errcheck
	}

	if store.snap != nil {
		store.snap.Cleanup()
	}
}

// ConnectOrCreateS3 implements the RepoManager interface, connects to a repo in an S3
// bucket or attempts to create one if connection is unsuccessful.
func (store *BlinkDiskPersister) ConnectOrCreateS3(bucketName, pathPrefix string) error {
	return store.snap.ConnectOrCreateS3(bucketName, pathPrefix)
}

// ConnectOrCreateFilesystem implements the RepoManager interface, connects to a repo in the filesystem
// or attempts to create one if connection is unsuccessful.
func (store *BlinkDiskPersister) ConnectOrCreateFilesystem(path string) error {
	return store.snap.ConnectOrCreateFilesystem(path)
}

const metadataStoreFileName = "metadata-store-latest"

// ConnectOrCreateS3WithServer implements the RepoManager interface, creates a server
// connects it a repo in an S3 bucket and creates a client to perform operations.
func (store *BlinkDiskPersister) ConnectOrCreateS3WithServer(serverAddr, bucketName, pathPrefix string) (*exec.Cmd, string, error) {
	return store.snap.ConnectOrCreateS3WithServer(serverAddr, bucketName, pathPrefix)
}

// ConnectOrCreateFilesystemWithServer implements the RepoManager interface, creates a server
// connects it a repo in the filesystem and creates a client to perform operations.
func (store *BlinkDiskPersister) ConnectOrCreateFilesystemWithServer(repoPath, serverAddr string) (*exec.Cmd, string, error) {
	return store.snap.ConnectOrCreateFilesystemWithServer(repoPath, serverAddr)
}

// LoadMetadata implements the DataPersister interface, restores the latest
// snapshot from the blinkdisk repository and decodes its contents, populating
// its metadata on the snapshots residing in the target test repository.
func (store *BlinkDiskPersister) LoadMetadata() error {
	snapIDs, err := store.snap.ListSnapshots()
	if err != nil {
		return err
	}

	if len(snapIDs) == 0 {
		return nil // No snapshot IDs found in repository
	}

	lastSnapID := snapIDs[len(snapIDs)-1]

	err = store.snap.RestoreSnapshot(lastSnapID, store.persistenceDir)
	if err != nil {
		return err
	}

	metadataPath := filepath.Join(store.persistenceDir, metadataStoreFileName)

	defer os.Remove(metadataPath) //nolint:errcheck

	f, err := os.Open(metadataPath) //nolint:gosec
	if err != nil {
		return err
	}

	err = json.NewDecoder(f).Decode(&(store.Simple))
	if err != nil {
		return err
	}

	return nil
}

// GetPersistDir returns the path to the directory that will be persisted
// as a snapshot to the blinkdisk repository.
func (store *BlinkDiskPersister) GetPersistDir() string {
	return store.persistenceDir
}

// FlushMetadata implements the DataPersister interface, flushing the local
// metadata on the target test repo's snapshots to the metadata BlinkDisk repository
// as a snapshot create.
func (store *BlinkDiskPersister) FlushMetadata() error {
	metadataPath := filepath.Join(store.persistenceDir, metadataStoreFileName)

	f, err := os.Create(metadataPath)
	if err != nil {
		return err
	}

	defer func() {
		f.Close()               //nolint:errcheck
		os.Remove(metadataPath) //nolint:errcheck
	}()

	err = json.NewEncoder(f).Encode(store.Simple)
	if err != nil {
		return err
	}

	_, err = store.snap.CreateSnapshot(store.persistenceDir)
	if err != nil {
		return err
	}

	return nil
}
