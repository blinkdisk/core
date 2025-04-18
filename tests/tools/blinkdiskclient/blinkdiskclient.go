//go:build darwin || (linux && amd64)
// +build darwin linux,amd64

// Package blinkdiskclient provides a client to interact with a BlinkDisk repo.
package blinkdiskclient

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/fs"
	"github.com/blinkdisk/core/fs/virtualfs"
	"github.com/blinkdisk/core/internal/units"
	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/blob"
	"github.com/blinkdisk/core/repo/blob/filesystem"
	"github.com/blinkdisk/core/repo/blob/s3"
	"github.com/blinkdisk/core/repo/content"
	"github.com/blinkdisk/core/snapshot"
	"github.com/blinkdisk/core/snapshot/policy"
	"github.com/blinkdisk/core/snapshot/snapshotfs"
	"github.com/blinkdisk/core/snapshot/upload"
	"github.com/blinkdisk/core/tests/robustness"
)

// BlinkDiskClient uses a BlinkDisk repo to create, restore, and delete snapshots.
type BlinkDiskClient struct {
	configPath string
	pw         string
}

const (
	configFileName           = "config"
	password                 = "kj13498po&_EXAMPLE" //nolint:gosec
	s3Endpoint               = "s3.amazonaws.com"
	awsAccessKeyIDEnvKey     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKeyEnvKey = "AWS_SECRET_ACCESS_KEY" //nolint:gosec
	dataFileName             = "data"
)

// NewBlinkDiskClient returns a new BlinkDiskClient.
func NewBlinkDiskClient(basePath string) *BlinkDiskClient {
	return &BlinkDiskClient{
		configPath: filepath.Join(basePath, configFileName),
		pw:         password,
	}
}

// CreateOrConnectRepo creates a new BlinkDisk repo or connects to an existing one if possible.
func (kc *BlinkDiskClient) CreateOrConnectRepo(ctx context.Context, repoDir, bucketName string) error {
	st, err := kc.getStorage(ctx, repoDir, bucketName)
	if err != nil {
		return err
	}

	if iErr := repo.Initialize(ctx, st, &repo.NewRepositoryOptions{}, kc.pw); iErr != nil {
		if !errors.Is(iErr, repo.ErrAlreadyInitialized) {
			return errors.Wrap(iErr, "repo is already initialized")
		}

		log.Println("connecting to existing repository")
	}

	if iErr := repo.Connect(ctx, kc.configPath, st, kc.pw, &repo.ConnectOptions{}); iErr != nil {
		return errors.Wrap(iErr, "error connecting to repository")
	}

	return errors.Wrap(err, "unable to open repository")
}

// SetCacheLimits sets cache size limits to the already connected repository.
func (kc *BlinkDiskClient) SetCacheLimits(ctx context.Context, repoDir, bucketName string, cacheOpts *content.CachingOptions) error {
	err := repo.SetCachingOptions(ctx, kc.configPath, cacheOpts)
	if err != nil {
		return err
	}

	cacheOptsObtained, err := repo.GetCachingOptions(ctx, kc.configPath)
	if err != nil {
		return err
	}

	log.Println("content cache size:", cacheOptsObtained.ContentCacheSizeLimitBytes)
	log.Println("metadata cache size:", cacheOptsObtained.MetadataCacheSizeLimitBytes)

	return nil
}

// SnapshotCreate creates a snapshot for the given path.
func (kc *BlinkDiskClient) SnapshotCreate(ctx context.Context, key string, val []byte) error {
	r, err := repo.Open(ctx, kc.configPath, kc.pw, &repo.Options{})
	if err != nil {
		return errors.Wrap(err, "cannot open repository")
	}

	ctx, rw, err := r.NewWriter(ctx, repo.WriteSessionOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot get new repository writer")
	}

	si := kc.getSourceInfoFromKey(r, key)

	policyTree, err := policy.TreeForSource(ctx, r, si)
	if err != nil {
		return errors.Wrap(err, "cannot get policy tree for source")
	}

	source := kc.getSourceForKeyVal(key, val)
	u := upload.NewUploader(rw)

	man, err := u.Upload(ctx, source, policyTree, si)
	if err != nil {
		return errors.Wrap(err, "cannot get manifest")
	}

	log.Printf("snapshotting %v", units.BytesString(atomic.LoadInt64(&man.Stats.TotalFileSize)))

	if _, err := snapshot.SaveSnapshot(ctx, rw, man); err != nil {
		return errors.Wrap(err, "cannot save snapshot")
	}

	if err := rw.Flush(ctx); err != nil {
		return err
	}

	return r.Close(ctx)
}

// SnapshotRestore restores the latest snapshot for the given path.
func (kc *BlinkDiskClient) SnapshotRestore(ctx context.Context, key string) ([]byte, error) {
	r, err := repo.Open(ctx, kc.configPath, kc.pw, &repo.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot open repository")
	}

	mans, err := kc.getSnapshotsFromKey(ctx, r, key)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get snapshots from key")
	}

	man := kc.latestManifest(mans)
	rootOIDWithPath := man.RootObjectID().String() + "/" + dataFileName

	oid, err := snapshotfs.ParseObjectIDWithPath(ctx, r, rootOIDWithPath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse object ID %s", rootOIDWithPath)
	}

	or, err := r.OpenObject(ctx, oid)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open object %s", oid)
	}

	val, err := io.ReadAll(or)
	if err != nil {
		return nil, err
	}

	log.Printf("restored %v", units.BytesString(len(val)))

	if err := r.Close(ctx); err != nil {
		return nil, err
	}

	return val, nil
}

// SnapshotDelete deletes all snapshots for a given path.
func (kc *BlinkDiskClient) SnapshotDelete(ctx context.Context, key string) error {
	r, err := repo.Open(ctx, kc.configPath, kc.pw, &repo.Options{})
	if err != nil {
		return errors.Wrap(err, "cannot open repository")
	}

	ctx, rw, err := r.NewWriter(ctx, repo.WriteSessionOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot get new repository writer")
	}

	mans, err := kc.getSnapshotsFromKey(ctx, r, key)
	if err != nil {
		return errors.Wrap(err, "cannot get snapshots from key")
	}

	for _, man := range mans {
		if err := rw.DeleteManifest(ctx, man.ID); err != nil {
			return errors.Wrap(err, "cannot delete manifest")
		}
	}

	if err := rw.Flush(ctx); err != nil {
		return err
	}

	return r.Close(ctx)
}

func (kc *BlinkDiskClient) getStorage(ctx context.Context, repoDir, bucketName string) (st blob.Storage, err error) {
	if bucketName != "" {
		s3Opts := &s3.Options{
			BucketName:      bucketName,
			Prefix:          repoDir,
			Endpoint:        s3Endpoint,
			AccessKeyID:     os.Getenv(awsAccessKeyIDEnvKey),
			SecretAccessKey: os.Getenv(awsSecretAccessKeyEnvKey),
		}
		st, err = s3.New(ctx, s3Opts, false)
	} else {
		if iErr := os.MkdirAll(repoDir, 0o700); iErr != nil {
			return nil, errors.Wrap(iErr, "cannot create directory")
		}

		fsOpts := &filesystem.Options{
			Path: repoDir,
		}
		st, err = filesystem.New(ctx, fsOpts, false)
	}

	return st, errors.Wrap(err, "unable to get storage")
}

// getSourceForKeyVal creates a virtual directory for `key` that contains a single virtual file that
// reads its contents from `val`.
func (kc *BlinkDiskClient) getSourceForKeyVal(key string, val []byte) fs.Entry {
	return virtualfs.NewStaticDirectory(key, []fs.Entry{
		virtualfs.StreamingFileFromReader(dataFileName, io.NopCloser(bytes.NewReader(val))),
	})
}

func (kc *BlinkDiskClient) getSnapshotsFromKey(ctx context.Context, r repo.Repository, key string) ([]*snapshot.Manifest, error) {
	si := kc.getSourceInfoFromKey(r, key)

	manifests, err := snapshot.ListSnapshots(ctx, r, si)
	if err != nil {
		return nil, errors.Wrap(err, "cannot list snapshots")
	}

	if len(manifests) == 0 {
		return nil, robustness.ErrKeyNotFound
	}

	return manifests, nil
}

func (kc *BlinkDiskClient) getSourceInfoFromKey(r repo.Repository, key string) snapshot.SourceInfo {
	return snapshot.SourceInfo{
		Host:     r.ClientOptions().Hostname,
		UserName: r.ClientOptions().Username,
		Path:     key,
	}
}

func (kc *BlinkDiskClient) latestManifest(mans []*snapshot.Manifest) *snapshot.Manifest {
	latest := mans[0]

	for _, m := range mans {
		if m.StartTime.After(latest.StartTime) {
			latest = m
		}
	}

	return latest
}
