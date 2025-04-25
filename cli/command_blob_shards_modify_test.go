package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/repo/blob/sharded"
	"github.com/blinkdisk/core/tests/testenv"
)

func TestBlobShardsModify(t *testing.T) {
	env := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, testenv.NewInProcRunner(t))

	env.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", env.RepoDir)

	someQBlob := strings.Split(env.RunAndExpectSuccess(t, "blob", "list", "--prefix=q")[0], " ")[0]

	// verify default sharding is 1,3
	require.FileExists(t, filepath.Join(env.RepoDir, someQBlob[0:1], someQBlob[1:4], someQBlob[4:]+sharded.CompleteBlobSuffix))
	require.FileExists(t, filepath.Join(env.RepoDir, "blinkdisk.repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--default-shards=5,5", "--i-am-sure-blinkdisk-is-not-running")

	// verify new sharding is 5,5
	require.FileExists(t, filepath.Join(env.RepoDir, someQBlob[0:5], someQBlob[5:10], someQBlob[10:]+sharded.CompleteBlobSuffix))
	require.NoFileExists(t, filepath.Join(env.RepoDir, someQBlob[0:3], someQBlob[3:6], someQBlob[6:]+sharded.CompleteBlobSuffix))
	require.FileExists(t, filepath.Join(env.RepoDir, "blinkdisk.repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--unsharded-length=0", "--i-am-sure-blinkdisk-is-not-running")

	require.FileExists(t, filepath.Join(env.RepoDir, someQBlob[0:5], someQBlob[5:10], someQBlob[10:]+sharded.CompleteBlobSuffix))
	require.FileExists(t, filepath.Join(env.RepoDir, "blink/disk./repository.f"))
	require.NoFileExists(t, filepath.Join(env.RepoDir, "blinkdisk.repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=bli=2,,,2", "--i-am-sure-blinkdisk-is-not-running")
	require.FileExists(t, filepath.Join(env.RepoDir, "bl/in/kdisk.repository.f"))
	require.NoFileExists(t, filepath.Join(env.RepoDir, "blinkdisk.repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--remove-override=nosuchprefix", "--remove-override=bli", "--i-am-sure-blinkdisk-is-not-running")
	require.FileExists(t, filepath.Join(env.RepoDir, "blink/disk./repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--i-am-sure-blinkdisk-is-not-running")

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=bli=flat", "--i-am-sure-blinkdisk-is-not-running", "--dry-run")
	require.FileExists(t, filepath.Join(env.RepoDir, "blink/disk./repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=bli=flat", "--i-am-sure-blinkdisk-is-not-running")
	require.FileExists(t, filepath.Join(env.RepoDir, "blinkdisk.repository.f"))

	env.RunAndExpectSuccess(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=bli=4,4", "--i-am-sure-blinkdisk-is-not-running")
	require.FileExists(t, filepath.Join(env.RepoDir, "blin/kdis/k.repository.f"))

	// some invalid cases
	env.RunAndExpectFailure(t, "blob", "shards", "modify", "--path", env.RepoDir, "--default-shards=invalid", "--i-am-sure-blinkdisk-is-not-running")
	env.RunAndExpectFailure(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=x", "--i-am-sure-blinkdisk-is-not-running")
	env.RunAndExpectFailure(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=x=aaa", "--i-am-sure-blinkdisk-is-not-running")
	env.RunAndExpectFailure(t, "blob", "shards", "modify", "--path", env.RepoDir, "--override=2,-1", "--i-am-sure-blinkdisk-is-not-running")
}
