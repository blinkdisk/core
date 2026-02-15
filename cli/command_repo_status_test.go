package cli_test

import (
	"testing"

	"github.com/blinkdisk/core/cli"
	"github.com/blinkdisk/core/internal/testutil"
	"github.com/blinkdisk/core/tests/testenv"
)

func TestRepoStatusJSON(t *testing.T) {
	t.Parallel()

	e := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, testenv.NewInProcRunner(t))

	var rs cli.RepositoryStatus

	e.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", e.RepoDir)
	defer e.RunAndExpectSuccess(t, "repo", "disconnect")

	e.RunAndExpectSuccess(t, "repo", "status")
	testutil.MustParseJSONLines(t, e.RunAndExpectSuccess(t, "repo", "status", "--json"), &rs)
}
