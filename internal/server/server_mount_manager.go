package server

import (
	"context"
	"runtime"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/internal/mount"
	"github.com/blinkdisk/core/repo"
	"github.com/blinkdisk/core/repo/object"
	"github.com/blinkdisk/core/snapshot/snapshotfs"
)

func (s *Server) getMountController(ctx context.Context, rep repo.Repository, oid object.ID, createIfNotFound bool) (mount.Controller, error) {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	c := s.mounts[oid]
	if c != nil {
		return c, nil
	}

	if !createIfNotFound {
		return nil, nil
	}

	log(ctx).Debugf("mount controller for %v not found, starting", oid)

	webdav := false
	if runtime.GOOS == "darwin" {
		webdav = true
	}

    c, err := mount.Directory(ctx, snapshotfs.DirectoryEntry(rep, oid, nil), "*", mount.Options{PreferWebDAV: webdav})
	if err != nil {
		return nil, errors.Wrap(err, "unable to mount")
	}

	s.mounts[oid] = c

	return c, nil
}

func (s *Server) listMounts() map[object.ID]mount.Controller {
	s.serverMutex.RLock()
	defer s.serverMutex.RUnlock()

	result := map[object.ID]mount.Controller{}

	for oid, c := range s.mounts {
		result[oid] = c
	}

	return result
}

func (s *Server) deleteMount(oid object.ID) {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	delete(s.mounts, oid)
}

// +checklocks:s.serverMutex
func (s *Server) unmountAllLocked(ctx context.Context) {
	for oid, c := range s.mounts {
		if err := c.Unmount(ctx); err != nil {
			log(ctx).Errorf("unable to unmount %v", oid)
		}

		delete(s.mounts, oid)
	}
}
