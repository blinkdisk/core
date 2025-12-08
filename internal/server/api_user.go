package server

import (
	"context"

	"github.com/blinkdisk/core/internal/serverapi"
	"github.com/blinkdisk/core/repo"
)

func handleCurrentUser(ctx context.Context, _ requestContext) (any, *apiError) {
	return serverapi.CurrentUserResponse{
		Username: repo.GetDefaultUserName(ctx),
		Hostname: repo.GetDefaultHostName(ctx),
	}, nil
}
