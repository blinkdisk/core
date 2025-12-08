//go:build darwin || (linux && amd64)

package robustness

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/blinkdisk/core/tests/robustness"
	"github.com/blinkdisk/core/tests/robustness/engine"
	"github.com/blinkdisk/core/tests/robustness/fiofilewriter"
	"github.com/blinkdisk/core/tests/robustness/snapmeta"
	"github.com/blinkdisk/core/tests/tools/fio"
	"github.com/blinkdisk/core/tests/tools/blinkdiskrunner"
)

var eng *engine.Engine // for use in the test functions

const (
	dataSubPath     = "robustness-data"
	metadataSubPath = "robustness-metadata"
	defaultTestDur  = 5 * time.Minute
)

var (
	randomizedTestDur = flag.Duration("rand-test-duration", defaultTestDur, "Set the duration for the randomized test")
	repoPathPrefix    = flag.String("repo-path-prefix", "", "Point the robustness tests at this path prefix")
)

func TestMain(m *testing.M) {
	flag.Parse()

	dataRepoPath := path.Join(*repoPathPrefix, dataSubPath)
	metadataRepoPath := path.Join(*repoPathPrefix, metadataSubPath)

	ctx := context.Background()

	th := &blinkdiskRobustnessTestHarness{}
	th.init(ctx, dataRepoPath, metadataRepoPath)
	eng = th.engine

	// Restore a random snapshot into the data directory
	if _, err := eng.ExecAction(ctx, engine.RestoreIntoDataDirectoryActionKey, nil); err != nil && !errors.Is(err, robustness.ErrNoOp) {
		th.cleanup(ctx)
		log.Fatalln("Error restoring into the data directory:", err)
	}

	// Upgrade the repository format version if the env var is set
	if os.Getenv("UPGRADE_REPOSITORY_FORMAT_VERSION") == "ON" {
		log.Print("Upgrading the repository.")

		rs, err := th.snapshotter.GetRepositoryStatus()
		exitOnError("failed to get repository status before upgrade", err)

		prev := rs.ContentFormat.Version

		log.Println("Old repository format:", prev)
		th.snapshotter.UpgradeRepository()

		rs, err = th.snapshotter.GetRepositoryStatus()
		exitOnError("failed to get repository status after upgrade", err)

		curr := rs.ContentFormat.Version
		log.Println("Upgraded repository format:", curr)

		// Reset the env variable.
		os.Setenv("BLINKDISK_UPGRADE_LOCK_ENABLED", "")
	}

	// run the tests
	result := m.Run()

	err := th.cleanup(ctx)
	exitOnError("Could not clean up after engine execution", err)

	os.Exit(result)
}

type blinkdiskRobustnessTestHarness struct {
	dataRepoPath string
	metaRepoPath string
	baseDirPath  string

	fileWriter  *fiofilewriter.FileWriter
	snapshotter *snapmeta.BlinkDiskSnapshotter
	persister   *snapmeta.BlinkDiskPersisterLight
	upgrader    *blinkdiskrunner.BlinkDiskSnapshotter
	engine      *engine.Engine

	skipTest bool
}

func (th *blinkdiskRobustnessTestHarness) init(ctx context.Context, dataRepoPath, metaRepoPath string) {
	th.dataRepoPath = dataRepoPath
	th.metaRepoPath = metaRepoPath

	// the initialization state machine is linear and bails out on first failure
	if th.makeBaseDir() && th.getFileWriter() && th.getSnapshotter() &&
		th.getPersister() && th.getEngine() && th.getUpgrader() {
		return // success!
	}

	th.cleanup(ctx)

	if th.skipTest {
		os.Exit(0)
	}

	os.Exit(1)
}

func (th *blinkdiskRobustnessTestHarness) makeBaseDir() bool {
	baseDir, err := os.MkdirTemp("", "engine-data-")
	if err != nil {
		log.Println("Error creating temp dir:", err)
		return false
	}

	th.baseDirPath = baseDir

	return true
}

func (th *blinkdiskRobustnessTestHarness) getFileWriter() bool {
	fw, err := fiofilewriter.New()
	if err != nil {
		if errors.Is(err, fio.ErrEnvNotSet) {
			log.Println("Skipping robustness tests because FIO environment is not set")

			th.skipTest = true
		} else {
			log.Println("Error creating fio FileWriter:", err)
		}

		return false
	}

	th.fileWriter = fw

	return true
}

func (th *blinkdiskRobustnessTestHarness) getSnapshotter() bool {
	ks, err := snapmeta.NewSnapshotter(th.baseDirPath)
	if err != nil {
		if errors.Is(err, blinkdiskrunner.ErrExeVariableNotSet) {
			log.Println("Skipping robustness tests because BLINKDISK_EXE is not set")

			th.skipTest = true
		} else {
			log.Println("Error creating blinkdisk Snapshotter:", err)
		}

		return false
	}

	th.snapshotter = ks

	if err = ks.ConnectOrCreateRepo(th.dataRepoPath); err != nil {
		log.Println("Error initializing blinkdisk Snapshotter:", err)
		return false
	}

	return true
}

func (th *blinkdiskRobustnessTestHarness) getPersister() bool {
	kp, err := snapmeta.NewPersisterLight(th.baseDirPath)
	if err != nil {
		log.Println("Error creating blinkdisk Persister:", err)
		return false
	}

	th.persister = kp

	if err = kp.ConnectOrCreateRepo(th.metaRepoPath); err != nil {
		log.Println("Error initializing blinkdisk Persister:", err)
		return false
	}

	return true
}

func (th *blinkdiskRobustnessTestHarness) getEngine() bool {
	args := &engine.Args{
		MetaStore:        th.persister,
		TestRepo:         th.snapshotter,
		FileWriter:       th.fileWriter,
		WorkingDir:       th.baseDirPath,
		SyncRepositories: true,
	}

	eng, err := engine.New(args) //nolint:govet
	if err != nil {
		log.Println("Error on engine creation:", err)
		return false
	}

	// Initialize the engine, connecting it to the repositories.
	// Note that th.engine is not yet set so that metadata will not be
	// flushed on cleanup in case there was an issue while loading.
	err = eng.Init(context.Background())
	if err != nil {
		log.Println("Error initializing engine for S3:", err)
		return false
	}

	th.engine = eng

	return true
}

func (th *blinkdiskRobustnessTestHarness) cleanup(ctx context.Context) (retErr error) {
	os.Setenv("BLINKDISK_UPGRADE_LOCK_ENABLED", "")

	if th.engine != nil {
		retErr = th.engine.Shutdown(ctx)
	}

	if th.persister != nil {
		th.persister.Cleanup()
	}

	if th.snapshotter != nil {
		if sc := th.snapshotter.ServerCmd(); sc != nil {
			if err := sc.Process.Signal(syscall.SIGTERM); err != nil {
				log.Println("Warning: Failed to send termination signal to blinkdisk server process:", err)
			}
		}

		th.snapshotter.Cleanup()
	}

	if th.fileWriter != nil {
		th.fileWriter.Cleanup()
	}

	if th.baseDirPath != "" {
		os.RemoveAll(th.baseDirPath)
	}

	return retErr
}

func (th *blinkdiskRobustnessTestHarness) getUpgrader() bool {
	ks, err := blinkdiskrunner.NewBlinkDiskSnapshotter(th.baseDirPath)
	if err != nil {
		if errors.Is(err, blinkdiskrunner.ErrExeVariableNotSet) {
			log.Println("Skipping robustness tests because BLINKDISK_EXE is not set")

			th.skipTest = true
		} else {
			log.Println("Error creating blinkdisk Upgrader:", err)
		}

		return false
	}

	th.upgrader = ks

	return true
}

func exitOnError(msg string, err error) {
	if err != nil {
		log.Fatal(msg, ": ", err.Error())
	}
}
