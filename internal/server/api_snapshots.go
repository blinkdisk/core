package server

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/internal/serverapi"
	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/manifest"
	"github.com/blinkdisk/core/snapshot"
	"github.com/blinkdisk/core/snapshot/policy"
)

func handleListSnapshots(ctx context.Context, rc requestContext) (any, *apiError) {
	si := getSnapshotSourceFromURL(rc.req.URL)

	manifestIDs, err := snapshot.ListSnapshotManifests(ctx, rc.rep, &si, nil)
	if err != nil {
		return nil, internalServerError(err)
	}

	manifests, err := snapshot.LoadSnapshots(ctx, rc.rep, manifestIDs)
	if err != nil {
		return nil, internalServerError(err)
	}

	manifests = snapshot.SortByTime(manifests, false)

	resp := &serverapi.SnapshotsResponse{
		Snapshots: []*serverapi.Snapshot{},
	}

	pol, _, _, err := policy.GetEffectivePolicy(ctx, rc.rep, si)
	if err == nil {
		pol.RetentionPolicy.ComputeRetentionReasons(manifests)
	}

	for _, m := range manifests {
		resp.Snapshots = append(resp.Snapshots, convertSnapshotManifest(m))
	}

	resp.UnfilteredCount = len(resp.Snapshots)

	if rc.queryParam("all") == "" {
		resp.Snapshots = uniqueSnapshots(resp.Snapshots)
		resp.UniqueCount = len(resp.Snapshots)
	} else {
		resp.UniqueCount = len(uniqueSnapshots(resp.Snapshots))
	}

	return resp, nil
}

func handleDeleteSnapshots(ctx context.Context, rc requestContext) (any, *apiError) {
	var req serverapi.DeleteSnapshotsRequest

	if err := json.Unmarshal(rc.body, &req); err != nil {
		return nil, requestError(serverapi.ErrorMalformedRequest, "malformed request")
	}

	sm := rc.srv.snapshotAllSourceManagers()[req.SourceInfo]
	if sm == nil {
		return nil, requestError(serverapi.ErrorNotFound, "unknown source")
	}

	// stop source manager and remove from map
	if req.DeleteSourceAndPolicy {
		if !rc.srv.deleteSourceManager(ctx, req.SourceInfo) {
			return nil, requestError(serverapi.ErrorNotFound, "unknown source")
		}
	}

	if err := repo.WriteSession(ctx, rc.rep, repo.WriteSessionOptions{
		Purpose: "DeleteSnapshots",
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		var manifestIDs []manifest.ID

		if req.DeleteSourceAndPolicy {
			mans, err := snapshot.ListSnapshotManifests(ctx, w, &req.SourceInfo, nil)
			if err != nil {
				return errors.Wrap(err, "unable to list snapshots")
			}

			manifestIDs = mans
		} else {
			snaps, err := snapshot.LoadSnapshots(ctx, w, req.SnapshotManifestIDs)
			if err != nil {
				return errors.Wrap(err, "unable to load snapshots")
			}

			for _, sn := range snaps {
				if sn.Source != req.SourceInfo {
					return errors.New("source info does not match snapshot source")
				}
			}

			manifestIDs = req.SnapshotManifestIDs
		}

		for _, m := range manifestIDs {
			if err := w.DeleteManifest(ctx, m); err != nil {
				return errors.Wrap(err, "unable to delete snapshot")
			}
		}

		if req.DeleteSourceAndPolicy {
			if err := policy.RemovePolicy(ctx, w, req.SourceInfo); err != nil {
				return errors.Wrap(err, "unable to remove policy")
			}
		}

		return nil
	}); err != nil {
		// if source deletion failed, refresh the repository to rediscover the source
		rc.srv.Refresh()

		return nil, internalServerError(err)
	}

	return &serverapi.Empty{}, nil
}

func handleEditSnapshots(ctx context.Context, rc requestContext) (any, *apiError) {
	var req serverapi.EditSnapshotsRequest

	if err := json.Unmarshal(rc.body, &req); err != nil {
		return nil, requestError(serverapi.ErrorMalformedRequest, "malformed request")
	}

	var snaps []*serverapi.Snapshot

	if err := repo.WriteSession(ctx, rc.rep, repo.WriteSessionOptions{
		Purpose: "EditSnapshots",
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		for _, id := range req.Snapshots {
			snap, err := snapshot.LoadSnapshot(ctx, w, id)
			if err != nil {
				return errors.Wrap(err, "unable to load snapshot")
			}

			changed := false

			if snap.UpdatePins(req.AddPins, req.RemovePins) {
				changed = true
			}

			if req.NewDescription != nil {
				changed = true
				snap.Description = *req.NewDescription
			}

			if changed {
				if err := snapshot.UpdateSnapshot(ctx, w, snap); err != nil {
					return errors.Wrap(err, "error updating snapshot")
				}
			}

			snaps = append(snaps, convertSnapshotManifest(snap))
		}

		return nil
	}); err != nil {
		return nil, internalServerError(err)
	}

	return snaps, nil
}

func handleMoveSnapshots(ctx context.Context, rc requestContext) (any, *apiError) {
	var req serverapi.MoveSnapshotsRequest

	if err := json.Unmarshal(rc.body, &req); err != nil {
		return nil, requestError(serverapi.ErrorMalformedRequest, "malformed request")
	}

	rw, ok := rc.rep.(repo.RepositoryWriter)
	if !ok {
		return nil, repositoryNotWritableError()
	}

	if req.Source == "" {
		return nil, requestError(serverapi.ErrorMalformedRequest, "source is required")
	}

	if req.Destination == "" {
		return nil, requestError(serverapi.ErrorMalformedRequest, "destination is required")
	}

	si, err := snapshot.ParseSourceInfo(req.Source, rc.rep.ClientOptions().Hostname, rc.rep.ClientOptions().Username)
	if err != nil {
		return nil, requestError(serverapi.ErrorMalformedRequest, errors.Wrap(err, "invalid source").Error())
	}

	di, err := snapshot.ParseSourceInfo(req.Destination, rc.rep.ClientOptions().Hostname, rc.rep.ClientOptions().Username)
	if err != nil {
		return nil, requestError(serverapi.ErrorMalformedRequest, errors.Wrap(err, "invalid destination").Error())
	}

	if di.Path != "" && si.Path == "" {
		return nil, requestError(serverapi.ErrorMalformedRequest, "path specified on destination but not source")
	}

	if di.UserName != "" && si.UserName == "" {
		return nil, requestError(serverapi.ErrorMalformedRequest, "username specified on destination but not source")
	}

	if err := repo.WriteSession(ctx, rw, repo.WriteSessionOptions{
		Purpose: "MoveSnapshots",
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		srcSnapshots, err := snapshot.ListSnapshots(ctx, w, si)
		if err != nil {
			return errors.Wrap(err, "error listing source snapshots")
		}

		dstSnapshots, err := snapshot.ListSnapshots(ctx, w, di)
		if err != nil {
			return errors.Wrap(err, "error listing destination snapshots")
		}

		for _, manifest := range srcSnapshots {
			dstSource := getMoveDestination(manifest.Source, di)

			if dstSource == manifest.Source {
				userLog(ctx).Debugf("%v is the same as destination, ignoring", dstSource)
				continue
			}

			if snapshotExists(dstSnapshots, dstSource, manifest) {
				userLog(ctx).Infof("%v (%v) already exists - deleting source", dstSource, manifest.StartTime.ToTime())

				if err := w.DeleteManifest(ctx, manifest.ID); err != nil {
					return errors.Wrap(err, "unable to delete source manifest")
				}

				continue
			}

			srcID := manifest.ID

			userLog(ctx).Infof("moving %v (%v) => %v", manifest.Source, manifest.StartTime.ToTime(), dstSource)

			manifest.ID = ""
			manifest.Source = dstSource

			if _, err := snapshot.SaveSnapshot(ctx, w, manifest); err != nil {
				return errors.Wrap(err, "unable to save snapshot")
			}

			if err := w.DeleteManifest(ctx, srcID); err != nil {
				return errors.Wrap(err, "unable to delete source manifest")
			}
		}

		return nil
	}); err != nil {
		return nil, internalServerError(err)
	}

	return &serverapi.Empty{}, nil
}

func getMoveDestination(source, overrides snapshot.SourceInfo) snapshot.SourceInfo {
	dst := source

	if overrides.Host != "" {
		dst.Host = overrides.Host
	}

	if overrides.UserName != "" {
		dst.UserName = overrides.UserName
	}

	if overrides.Path != "" {
		dst.Path = overrides.Path
	}

	return dst
}

func snapshotExists(snaps []*snapshot.Manifest, src snapshot.SourceInfo, srcManifest *snapshot.Manifest) bool {
	for _, s := range snaps {
		if src != s.Source {
			continue
		}

		if sameSnapshot(srcManifest, s) {
			return true
		}
	}

	return false
}

func sameSnapshot(a, b *snapshot.Manifest) bool {
	if !a.StartTime.Equal(b.StartTime) {
		return false
	}

	if a.RootObjectID() != b.RootObjectID() {
		return false
	}

	return true
}

func forAllSourceManagersMatchingURLFilter(ctx context.Context, managers map[snapshot.SourceInfo]*sourceManager, c func(s *sourceManager, ctx context.Context) serverapi.SourceActionResponse, values url.Values) (any, *apiError) {
	resp := &serverapi.MultipleSourceActionResponse{
		Sources: map[string]serverapi.SourceActionResponse{},
	}

	for src, mgr := range managers {
		if mgr.isRunningReadOnly() {
			continue
		}

		if !sourceMatchesURLFilter(src, values) {
			continue
		}

		resp.Sources[src.String()] = c(mgr, ctx)
	}

	if len(resp.Sources) == 0 {
		return nil, notFoundError("no source matching the provided filters")
	}

	return resp, nil
}

func handleUpload(ctx context.Context, rc requestContext) (any, *apiError) {
	return forAllSourceManagersMatchingURLFilter(ctx, rc.srv.snapshotAllSourceManagers(), (*sourceManager).upload, rc.req.URL.Query())
}

func handleCancel(ctx context.Context, rc requestContext) (any, *apiError) {
	return forAllSourceManagersMatchingURLFilter(ctx, rc.srv.snapshotAllSourceManagers(), (*sourceManager).cancel, rc.req.URL.Query())
}

func handlePause(ctx context.Context, rc requestContext) (any, *apiError) {
	return forAllSourceManagersMatchingURLFilter(ctx, rc.srv.snapshotAllSourceManagers(), (*sourceManager).pause, rc.req.URL.Query())
}

func handleResume(ctx context.Context, rc requestContext) (any, *apiError) {
	return forAllSourceManagersMatchingURLFilter(ctx, rc.srv.snapshotAllSourceManagers(), (*sourceManager).resume, rc.req.URL.Query())
}

func uniqueSnapshots(rows []*serverapi.Snapshot) []*serverapi.Snapshot {
	result := []*serverapi.Snapshot{}
	resultByRootEntry := map[string]*serverapi.Snapshot{}

	for _, r := range rows {
		last := resultByRootEntry[r.RootEntry]
		if last == nil {
			result = append(result, r)
			resultByRootEntry[r.RootEntry] = r
		} else {
			last.RetentionReasons = append(last.RetentionReasons, r.RetentionReasons...)
			last.Pins = append(last.Pins, r.Pins...)
		}
	}

	for _, r := range result {
		r.RetentionReasons = policy.CompactRetentionReasons(r.RetentionReasons)
		r.Pins = policy.CompactPins(r.Pins)
	}

	return result
}

func sourceMatchesURLFilter(src snapshot.SourceInfo, query url.Values) bool {
	if v := query.Get("host"); v != "" && src.Host != v {
		return false
	}

	if v := query.Get("userName"); v != "" && src.UserName != v {
		return false
	}

	if v := query.Get("path"); v != "" && src.Path != v {
		return false
	}

	return true
}

func convertSnapshotManifest(m *snapshot.Manifest) *serverapi.Snapshot {
	e := &serverapi.Snapshot{
		ID:               m.ID,
		Description:      m.Description,
		StartTime:        m.StartTime,
		EndTime:          m.EndTime,
		IncompleteReason: m.IncompleteReason,
		RootEntry:        m.RootObjectID().String(),
		RetentionReasons: append([]string{}, m.RetentionReasons...),
		Pins:             append([]string{}, m.Pins...),
	}

	if re := m.RootEntry; re != nil {
		e.Summary = re.DirSummary
	}

	return e
}
