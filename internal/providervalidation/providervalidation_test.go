package providervalidation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/internal/blobtesting"
	"github.com/blinkdisk/core/internal/providervalidation"
	"github.com/blinkdisk/core/internal/testlogging"
	"github.com/blinkdisk/core/repo/blob/filesystem"
)

func TestProviderValidation(t *testing.T) {
	ctx := testlogging.Context(t)
	st, err := filesystem.New(ctx, &filesystem.Options{
		Path: t.TempDir(),
	}, false)
	require.NoError(t, err)

	opt := blobtesting.TestValidationOptions
	opt.ConcurrencyTestDuration = 3 * time.Second
	require.NoError(t, providervalidation.ValidateProvider(ctx, st, opt))
}
