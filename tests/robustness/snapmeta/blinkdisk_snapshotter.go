//go:build darwin || (linux && amd64)
// +build darwin linux,amd64

package snapmeta

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/blinkdisk/core/cli"
	"github.com/blinkdisk/core/internal/clock"
	"github.com/blinkdisk/core/tests/robustness"
	"github.com/blinkdisk/core/tests/tools/fswalker"
)

// BlinkDiskSnapshotter wraps the functionality to connect to a blinkdisk repository with
// the fswalker WalkCompare.
type BlinkDiskSnapshotter struct {
	comparer *fswalker.WalkCompare
	blinkdiskConnector
}

// BlinkDiskSnapshotter implements robustness.Snapshotter.
var _ robustness.Snapshotter = (*BlinkDiskSnapshotter)(nil)

// NewSnapshotter returns a BlinkDisk based Snapshotter.
// ConnectOrCreateRepo must be invoked to enable the interface.
func NewSnapshotter(baseDirPath string) (*BlinkDiskSnapshotter, error) {
	ks := &BlinkDiskSnapshotter{
		comparer: fswalker.NewWalkCompare(),
	}

	if err := ks.initializeConnector(baseDirPath); err != nil {
		return nil, err
	}

	return ks, nil
}

// ConnectOrCreateRepo makes the Snapshotter ready for use.
func (ks *BlinkDiskSnapshotter) ConnectOrCreateRepo(repoPath string) error {
	if err := ks.connectOrCreateRepo(repoPath); err != nil {
		return err
	}

	_, _, err := ks.snap.Run("policy", "set", "--global", "--keep-latest", strconv.Itoa(1<<31-1), "--compression", "s2-default")

	return err
}

// ConnectClient should be called by a client to connect itself to the server
// using the given cert fingerprint.
func (ks *BlinkDiskSnapshotter) ConnectClient(fingerprint, user string) error {
	return ks.connectClient(fingerprint, user)
}

// DisconnectClient should be called by a client to disconnect itself from the server.
func (ks *BlinkDiskSnapshotter) DisconnectClient(user string) {
	if err := ks.snap.DisconnectClient(); err != nil {
		log.Printf("Error disconnecting %s from server: %v\n", user, err)
	}
}

// AuthorizeClient should be called by a server to add a client to the server's
// user list.
func (ks *BlinkDiskSnapshotter) AuthorizeClient(user string) error {
	return ks.authorizeClient(user)
}

// RemoveClient should be called by a server to remove a client from its user list.
func (ks *BlinkDiskSnapshotter) RemoveClient(user string) {
	if err := ks.snap.RemoveClient(user, defaultHost); err != nil {
		log.Printf("Error removing %s from server: %v\n", user, err)
	}
}

// ServerCmd returns the server command.
func (ks *BlinkDiskSnapshotter) ServerCmd() *exec.Cmd {
	return ks.serverCmd
}

// ServerFingerprint returns the cert fingerprint that is used to connect to the server.
func (ks *BlinkDiskSnapshotter) ServerFingerprint() string {
	return ks.serverFingerprint
}

// CreateSnapshot is part of Snapshotter.
func (ks *BlinkDiskSnapshotter) CreateSnapshot(ctx context.Context, sourceDir string, opts map[string]string) (snapID string, fingerprint []byte, snapStats *robustness.CreateSnapshotStats, err error) {
	fingerprint, err = ks.comparer.Gather(ctx, sourceDir, opts)
	if err != nil {
		return
	}

	ssStart := clock.Now()

	snapID, err = ks.snap.CreateSnapshot(sourceDir)
	if err != nil {
		return
	}

	ssEnd := clock.Now()

	snapStats = &robustness.CreateSnapshotStats{
		SnapStartTime: ssStart,
		SnapEndTime:   ssEnd,
	}

	return
}

// RestoreSnapshot restores the snapshot with the given ID to the provided restore directory. It returns
// fingerprint verification data of the restored snapshot directory.
func (ks *BlinkDiskSnapshotter) RestoreSnapshot(ctx context.Context, snapID, restoreDir string, opts map[string]string) (fingerprint []byte, err error) {
	err = ks.snap.RestoreSnapshot(snapID, restoreDir)
	if err != nil {
		return
	}

	return ks.comparer.Gather(ctx, restoreDir, opts)
}

// RestoreSnapshotCompare restores the snapshot with the given ID to the provided restore directory, then verifies the data
// that has been restored against the provided fingerprint validation data.
func (ks *BlinkDiskSnapshotter) RestoreSnapshotCompare(ctx context.Context, snapID, restoreDir string, validationData []byte, reportOut io.Writer, opts map[string]string) (err error) {
	err = ks.snap.RestoreSnapshot(snapID, restoreDir)
	if err != nil {
		return err
	}

	return ks.comparer.Compare(ctx, restoreDir, validationData, reportOut, opts)
}

// DeleteSnapshot is part of Snapshotter.
func (ks *BlinkDiskSnapshotter) DeleteSnapshot(ctx context.Context, snapID string, opts map[string]string) error {
	return ks.snap.DeleteSnapshot(snapID)
}

// RunGC is part of Snapshotter.
func (ks *BlinkDiskSnapshotter) RunGC(ctx context.Context, opts map[string]string) error {
	return ks.snap.RunGC()
}

// ListSnapshots is part of Snapshotter.
func (ks *BlinkDiskSnapshotter) ListSnapshots(ctx context.Context) ([]string, error) {
	return ks.snap.ListSnapshots()
}

// Run is part of Snapshotter.
func (ks *BlinkDiskSnapshotter) Run(args ...string) (stdout, stderr string, err error) {
	return ks.snap.Run(args...)
}

// ConnectOrCreateS3 TBD: remove this.
func (ks *BlinkDiskSnapshotter) ConnectOrCreateS3(bucketName, pathPrefix string) error {
	return nil
}

// ConnectOrCreateFilesystem TBD: remove this.
func (ks *BlinkDiskSnapshotter) ConnectOrCreateFilesystem(path string) error {
	return nil
}

// ConnectOrCreateS3WithServer TBD: remove this.
func (ks *BlinkDiskSnapshotter) ConnectOrCreateS3WithServer(serverAddr, bucketName, pathPrefix string) (*exec.Cmd, error) {
	//nolint:nilnil
	return nil, nil
}

// ConnectOrCreateFilesystemWithServer TBD: remove this.
func (ks *BlinkDiskSnapshotter) ConnectOrCreateFilesystemWithServer(serverAddr, repoPath string) (*exec.Cmd, error) {
	//nolint:nilnil
	return nil, nil
}

// Cleanup should be called before termination.
func (ks *BlinkDiskSnapshotter) Cleanup() {
	ks.snap.Cleanup()
}

// GetRepositoryStatus returns the repository status in JSON format.
func (ks *BlinkDiskSnapshotter) GetRepositoryStatus() (cli.RepositoryStatus, error) {
	var rs cli.RepositoryStatus

	a1, _, err := ks.snap.Run("repository", "status", "--json")
	if err != nil {
		return rs, err
	}

	if err := json.Unmarshal([]byte(a1), &rs); err != nil {
		return rs, err
	}

	return rs, nil
}

// UpgradeRepository upgrades the given blinkdisk repository
// from current format version to latest stable format version.
func (ks *BlinkDiskSnapshotter) UpgradeRepository() error {
	// This variable is also reset in cleanup function
	// in case the test fails
	os.Setenv("BLINKDISK_UPGRADE_LOCK_ENABLED", "1")

	_, _, err := ks.snap.Run("repository", "upgrade", "begin",
		"--upgrade-owner-id", "robustness-tests",
		"--io-drain-timeout", "30s", "--allow-unsafe-upgrade",
		"--status-poll-interval", "1s")

	// cleanup
	os.Setenv("BLINKDISK_UPGRADE_LOCK_ENABLED", "")

	return err
}
