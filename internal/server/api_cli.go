package server

import (
	"context"
	"os"
	"strings"

	"github.com/blinkdisk/core/internal/serverapi"
)

func handleCLIInfo(_ context.Context, rc requestContext) (any, *apiError) {
	executable, err := os.Executable()
	if err != nil {
		executable = "blinkdisk"
	}

	return &serverapi.CLIInfo{
		Executable: maybeQuote(executable) + " --config-file=" + maybeQuote(rc.srv.getOptions().ConfigFile) + "",
	}, nil
}

func maybeQuote(s string) string {
	if !strings.Contains(s, " ") {
		return s
	}

	return "\"" + s + "\""
}
