//go:build darwin || (linux && amd64)

package snapmeta

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/blinkdisk/core/repo/content"
	"github.com/blinkdisk/core/tests/robustness"
	"github.com/blinkdisk/core/tests/tools/blinkdiskclient"
)

// BlinkDiskPersisterLight is a wrapper for BlinkDiskClient that satisfies the Persister
// interface.
type BlinkDiskPersisterLight struct {
	kc            *blinkdiskclient.BlinkDiskClient
	keysInProcess map[string]bool
	c             *sync.Cond
	baseDir       string
}

var _ robustness.Persister = (*BlinkDiskPersisterLight)(nil)

// NewPersisterLight returns a new BlinkDiskPersisterLight.
func NewPersisterLight(baseDir string) (*BlinkDiskPersisterLight, error) {
	persistenceDir, err := os.MkdirTemp(baseDir, "blinkdisk-persistence-root-")
	if err != nil {
		return nil, err
	}

	return &BlinkDiskPersisterLight{
		kc:            blinkdiskclient.NewBlinkDiskClient(persistenceDir),
		keysInProcess: map[string]bool{},
		c:             sync.NewCond(&sync.Mutex{}),
		baseDir:       persistenceDir,
	}, nil
}

// ConnectOrCreateRepo creates a new BlinkDisk repo or connects to an existing one if possible.
func (kpl *BlinkDiskPersisterLight) ConnectOrCreateRepo(repoPath string) error {
	bucketName := os.Getenv(S3BucketNameEnvKey)
	return kpl.kc.CreateOrConnectRepo(context.Background(), repoPath, bucketName)
}

// SetCacheLimits sets to an existing one if possible.
func (kpl *BlinkDiskPersisterLight) SetCacheLimits(repoPath string, cacheOpts *content.CachingOptions) error {
	bucketName := os.Getenv(S3BucketNameEnvKey)

	err := kpl.kc.SetCacheLimits(context.Background(), repoPath, bucketName, cacheOpts)
	if err != nil {
		return err
	}

	return nil
}

// Store pushes the key value pair to the BlinkDisk repository.
func (kpl *BlinkDiskPersisterLight) Store(ctx context.Context, key string, val []byte) error {
	kpl.waitFor(key)
	defer kpl.doneWith(key)

	log.Println("pushing metadata for", key)

	return kpl.kc.SnapshotCreate(ctx, key, val)
}

// Load pulls the key value pair from the BlinkDisk repo and returns the value.
func (kpl *BlinkDiskPersisterLight) Load(ctx context.Context, key string) ([]byte, error) {
	kpl.waitFor(key)
	defer kpl.doneWith(key)

	log.Println("pulling metadata for", key)

	return kpl.kc.SnapshotRestore(ctx, key)
}

// Delete deletes all snapshots associated with the given key.
func (kpl *BlinkDiskPersisterLight) Delete(ctx context.Context, key string) error {
	kpl.waitFor(key)
	defer kpl.doneWith(key)

	log.Println("deleting metadata for", key)

	return kpl.kc.SnapshotDelete(ctx, key)
}

// LoadMetadata is a no-op. It is included to satisfy the Persister interface.
func (kpl *BlinkDiskPersisterLight) LoadMetadata() error {
	return nil
}

// FlushMetadata is a no-op. It is included to satisfy the Persister interface.
func (kpl *BlinkDiskPersisterLight) FlushMetadata() error {
	return nil
}

// GetPersistDir returns the persistence directory.
func (kpl *BlinkDiskPersisterLight) GetPersistDir() string {
	return kpl.baseDir
}

// Cleanup removes the persistence directory and closes the BlinkDisk repo.
func (kpl *BlinkDiskPersisterLight) Cleanup() {
	if err := os.RemoveAll(kpl.baseDir); err != nil {
		log.Println("cannot remove persistence dir")
	}
}

func (kpl *BlinkDiskPersisterLight) waitFor(key string) {
	kpl.c.L.Lock()

	for kpl.keysInProcess[key] {
		kpl.c.Wait()
	}

	kpl.keysInProcess[key] = true
	kpl.c.L.Unlock()
}

func (kpl *BlinkDiskPersisterLight) doneWith(key string) {
	kpl.c.L.Lock()
	delete(kpl.keysInProcess, key)
	kpl.c.L.Unlock()
	kpl.c.Signal()
}
