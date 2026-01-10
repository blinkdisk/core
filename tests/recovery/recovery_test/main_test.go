//go:build darwin || (linux && amd64)

package recovery

import (
	"flag"
	"log"
	"os"
	"path"
	"testing"
)

const (
	dataSubPath = "recovery-data"
	dirPath     = "blinkdisk_dummy_repo"
	dataPath    = "crash-consistency-data"
)

var repoPathPrefix = flag.String("repo-path-prefix", "", "Point the robustness tests at this path prefix")

func TestMain(m *testing.M) {
	dataRepoPath := path.Join(*repoPathPrefix, dataSubPath)

	th := &blinkdiskRecoveryTestHarness{}
	th.init(dataRepoPath)

	// run the tests
	result := m.Run()

	os.Exit(result)
}

type blinkdiskRecoveryTestHarness struct {
	dataRepoPath string
}

func (th *blinkdiskRecoveryTestHarness) init(dataRepoPath string) {
	th.dataRepoPath = dataRepoPath

	blinkdiskExe := os.Getenv("BLINKDISK_EXE")
	if blinkdiskExe == "" {
		log.Println("Skipping recovery tests because BLINKDISK_EXE is not set")
		os.Exit(0)
	}
}
