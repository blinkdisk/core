package server_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/internal/apiclient"
	"github.com/blinkdisk/core/internal/clock"
	"github.com/blinkdisk/core/internal/serverapi"
	"github.com/blinkdisk/core/internal/testlogging"
	"github.com/blinkdisk/core/internal/uitask"
	"github.com/blinkdisk/core/snapshot"
	"github.com/blinkdisk/core/snapshot/policy"
)

func mustCreateSource(t *testing.T, cli *apiclient.BlinkDiskAPIClient, path string, pol *policy.Policy) {
	t.Helper()

	_, err := serverapi.CreateSnapshotSource(testlogging.Context(t), cli, &serverapi.CreateSnapshotSourceRequest{
		Path:   path,
		Policy: pol,
	})
	require.NoError(t, err)
}

func mustSetPolicy(t *testing.T, cli *apiclient.BlinkDiskAPIClient, si snapshot.SourceInfo, pol *policy.Policy) {
	t.Helper()

	require.NoError(t, serverapi.SetPolicy(testlogging.Context(t), cli, si, pol))
}

func mustListSources(t *testing.T, cli *apiclient.BlinkDiskAPIClient, match *snapshot.SourceInfo) []*serverapi.SourceStatus {
	t.Helper()

	resp, err := serverapi.ListSources(testlogging.Context(t), cli, match)
	require.NoError(t, err)

	return resp.Sources
}

func mustGetTask(t *testing.T, cli *apiclient.BlinkDiskAPIClient, taskID string) uitask.Info {
	t.Helper()

	resp, err := serverapi.GetTask(testlogging.Context(t), cli, taskID)
	require.NoError(t, err)

	return *resp
}

func mustListTasks(t *testing.T, cli *apiclient.BlinkDiskAPIClient) []uitask.Info {
	t.Helper()

	resp, err := serverapi.ListTasks(testlogging.Context(t), cli)
	require.NoError(t, err)

	return resp.Tasks
}

func mustGetLatestTask(t *testing.T, cli *apiclient.BlinkDiskAPIClient) uitask.Info {
	t.Helper()

	tl := mustListTasks(t, cli)
	require.NotEmpty(t, tl)

	return tl[0]
}

func waitForTask(t *testing.T, cli *apiclient.BlinkDiskAPIClient, taskID string, timeout time.Duration) uitask.Info {
	t.Helper()

	var lastInfo uitask.Info

	deadline := clock.Now().Add(timeout)
	for clock.Now().Before(deadline) {
		lastInfo = mustGetTask(t, cli, taskID)

		if lastInfo.Status.IsFinished() {
			return lastInfo
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("task %v did not complete in %v, last: %v", taskID, timeout, lastInfo)

	return lastInfo
}
