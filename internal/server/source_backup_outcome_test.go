package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/internal/clock"
	"github.com/kopia/kopia/internal/serverapi"
	"github.com/kopia/kopia/internal/uitask"
	"github.com/kopia/kopia/snapshot"
)

func TestSourceBackupOutcomePersistence(t *testing.T) {
	config := filepath.Join(t.TempDir(), "repository.config")
	src := snapshot.SourceInfo{Host: "laptop", UserName: "user", Path: "/photos"}
	store := newSourceBackupOutcomeStore(config, []byte("repository"), src)
	manager := &sourceManager{src: src, outcomeStore: store}
	now := clock.Now().UTC()
	task := uitask.Info{StartTime: now, EndTime: &now, Status: uitask.StatusFailed, ErrorMessage: "storage unavailable"}
	manager.recordBackupOutcome(context.Background(), task, &snapshot.Manifest{})
	failed, err := store.load()
	require.NoError(t, err)
	require.Equal(t, manager.lastBackupOutcome, failed)
	require.Equal(t, "FAILED", failed.Status)
	require.Equal(t, "storage unavailable", failed.ErrorMessage)

	for _, status := range []uitask.Status{uitask.StatusCanceled, uitask.StatusRunning, uitask.StatusCanceling} {
		task.Status = status
		manager.recordBackupOutcome(context.Background(), task, &snapshot.Manifest{})
		require.Equal(t, failed, manager.lastBackupOutcome)
	}
	task.Status = uitask.StatusSuccess
	manager.recordBackupOutcome(context.Background(), task, &snapshot.Manifest{})
	require.Equal(t, failed, manager.lastBackupOutcome)
	manager.recordBackupOutcome(context.Background(), task, &snapshot.Manifest{IncompleteReason: "canceled"})
	require.Equal(t, failed, manager.lastBackupOutcome)
	loaded, err := store.load()
	require.NoError(t, err)
	require.Equal(t, failed, loaded)

	task.ErrorMessage = ""
	manager.recordBackupOutcome(context.Background(), task, &snapshot.Manifest{RootEntry: &snapshot.DirEntry{}, Stats: snapshot.Stats{ErrorCount: 2}})
	partial, err := store.load()
	require.NoError(t, err)
	require.Equal(t, "PARTIAL", partial.Status)
	require.EqualValues(t, 2, partial.ErrorCount)
	require.NotEqual(t, failed.ID, partial.ID)

	manager.recordBackupOutcome(context.Background(), task, &snapshot.Manifest{RootEntry: &snapshot.DirEntry{}})
	success, err := store.load()
	require.NoError(t, err)
	require.Equal(t, "SUCCESS", success.Status)
	require.Empty(t, success.ErrorMessage)
	require.Zero(t, success.ErrorCount)

	other := src
	other.Path = "/documents"
	for _, isolated := range []sourceBackupOutcomeStore{
		newSourceBackupOutcomeStore(config, []byte("other-repository"), src),
		newSourceBackupOutcomeStore(config, []byte("repository"), other),
	} {
		outcome, err := isolated.load()
		require.NoError(t, err)
		require.Nil(t, outcome)
	}
	require.NoError(t, store.remove())
	require.NoError(t, store.remove())
	loaded, err = store.load()
	require.NoError(t, err)
	require.Nil(t, loaded)
}

func TestSourceBackupOutcomeStoreInvalidData(t *testing.T) {
	store := sourceBackupOutcomeStore{filename: filepath.Join(t.TempDir(), "outcome.json")}
	for _, data := range []string{"{", "{}", `{"id":"1","status":"RUNNING"}`} {
		require.NoError(t, os.WriteFile(store.filename, []byte(data), 0o600))
		outcome, err := store.load()
		require.Error(t, err)
		require.Nil(t, outcome)
	}
	disabled := newSourceBackupOutcomeStore("", nil, snapshot.SourceInfo{})
	require.NoError(t, disabled.save(&serverapi.SourceBackupOutcome{}))
	require.NoError(t, disabled.remove())
	outcome, err := disabled.load()
	require.NoError(t, err)
	require.Nil(t, outcome)
}
