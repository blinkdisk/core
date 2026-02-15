package cli_test

import (
	"testing"

	"github.com/blinkdisk/core/cli"
	"github.com/blinkdisk/core/internal/testutil"
	"github.com/blinkdisk/core/tests/testenv"
)

func TestMaintenanceInfoSimple(t *testing.T) {
	t.Parallel()

	e := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, testenv.NewInProcRunner(t))
	defer e.RunAndExpectSuccess(t, "repo", "disconnect")

	var mi cli.MaintenanceInfo

	e.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", e.RepoDir)
	e.RunAndExpectSuccess(t, "maintenance", "info")
	testutil.MustParseJSONLines(t, e.RunAndExpectSuccess(t, "maintenance", "info", "--json"), &mi)
}
