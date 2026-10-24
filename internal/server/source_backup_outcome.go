package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/kopia/kopia/internal/atomicfile"
	"github.com/kopia/kopia/internal/serverapi"
	"github.com/kopia/kopia/internal/uitask"
	"github.com/kopia/kopia/snapshot"
)

type sourceBackupOutcomeStore struct {
	filename string
}

func newSourceBackupOutcomeStore(configFile string, repositoryID []byte, src snapshot.SourceInfo) sourceBackupOutcomeStore {
	if configFile == "" {
		return sourceBackupOutcomeStore{}
	}

	identity, _ := json.Marshal(src)
	sourceHash := sha256.Sum256(identity)

	return sourceBackupOutcomeStore{filename: filepath.Join(
		configFile+".backup-outcomes", hex.EncodeToString(repositoryID), hex.EncodeToString(sourceHash[:])+".json",
	)}
}

func (s sourceBackupOutcomeStore) load() (*serverapi.SourceBackupOutcome, error) {
	if s.filename == "" {
		return nil, nil
	}

	data, err := os.ReadFile(atomicfile.MaybePrefixLongFilenameOnWindows(s.filename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "read backup outcome")
	}

	var outcome serverapi.SourceBackupOutcome
	if err := json.Unmarshal(data, &outcome); err != nil {
		return nil, errors.Wrap(err, "decode backup outcome")
	}

	if outcome.ID == "" || outcome.StartTime.IsZero() || outcome.EndTime.IsZero() ||
		(outcome.Status != "SUCCESS" && outcome.Status != "FAILED" && outcome.Status != "PARTIAL") {
		return nil, errors.New("invalid backup outcome")
	}

	return &outcome, nil
}

func (s sourceBackupOutcomeStore) save(outcome *serverapi.SourceBackupOutcome) error {
	if s.filename == "" {
		return nil
	}

	data, err := json.Marshal(outcome)
	if err != nil {
		return errors.Wrap(err, "encode backup outcome")
	}

	if err := os.MkdirAll(atomicfile.MaybePrefixLongFilenameOnWindows(filepath.Dir(s.filename)), 0o700); err != nil {
		return errors.Wrap(err, "create backup outcome directory")
	}

	return errors.Wrap(atomicfile.Write(s.filename, bytes.NewReader(data)), "write backup outcome")
}

func (s sourceBackupOutcomeStore) remove() error {
	if s.filename == "" {
		return nil
	}

	err := os.Remove(atomicfile.MaybePrefixLongFilenameOnWindows(s.filename))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return errors.Wrap(err, "remove backup outcome")
}

func (s *sourceManager) recordBackupOutcome(ctx context.Context, task uitask.Info, manifest *snapshot.Manifest) {
	if task.EndTime == nil || (task.Status != uitask.StatusFailed && task.Status != uitask.StatusSuccess) {
		return
	}

	// Canceled, limited, or skipped uploads do not resolve an existing problem.
	if task.Status == uitask.StatusSuccess && (manifest.RootEntry == nil || manifest.IncompleteReason != "") {
		return
	}

	outcome := &serverapi.SourceBackupOutcome{
		ID:           uuid.NewString(),
		Status:       string(task.Status),
		StartTime:    task.StartTime,
		EndTime:      *task.EndTime,
		ErrorMessage: task.ErrorMessage,
		ErrorCount:   manifest.Stats.ErrorCount,
	}
	if manifest.RootEntry != nil && manifest.RootEntry.DirSummary != nil {
		outcome.ErrorCount = max(outcome.ErrorCount, int32(manifest.RootEntry.DirSummary.FatalErrorCount))
	}
	if task.Status == uitask.StatusSuccess && outcome.ErrorCount > 0 {
		outcome.Status = "PARTIAL"
	}

	s.sourceMutex.Lock()
	s.lastBackupOutcome = outcome
	s.sourceMutex.Unlock()

	if err := s.outcomeStore.save(outcome); err != nil {
		userLog(ctx).Errorf("unable to persist backup outcome for %v: %v", s.src, err)
	}
}

func (s *sourceManager) removeBackupOutcome(ctx context.Context) {
	if err := s.outcomeStore.remove(); err != nil {
		userLog(ctx).Errorf("unable to remove backup outcome for %v: %v", s.src, err)
	}
}
