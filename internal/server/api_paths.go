package server

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/blinkdisk/core/internal/ospath"
	"github.com/blinkdisk/core/internal/serverapi"
	"github.com/blinkdisk/core/snapshot"
)

func handlePathResolve(_ context.Context, rc requestContext) (any, *apiError) {
	var req serverapi.ResolvePathRequest

	if err := json.Unmarshal(rc.body, &req); err != nil {
		return nil, requestError(serverapi.ErrorMalformedRequest, "malformed request body")
	}

	return &serverapi.ResolvePathResponse{
		SourceInfo: snapshot.SourceInfo{
			Path:     filepath.Clean(ospath.ResolveUserFriendlyPath(req.Path, true)),
			Host:     rc.rep.ClientOptions().Hostname,
			UserName: rc.rep.ClientOptions().Username,
		},
	}, nil
}
