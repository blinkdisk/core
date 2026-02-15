package repo

import (
	"github.com/blinkdisk/core/internal/cache"
	"github.com/blinkdisk/core/internal/metrics"
	"github.com/blinkdisk/core/repo/format"
	"github.com/blinkdisk/core/repo/hashing"
)

// immutableServerRepositoryParameters contains immutable parameters shared between HTTP and GRPC clients.
type immutableServerRepositoryParameters struct {
	h               hashing.HashFunc
	objectFormat    format.ObjectFormat
	cliOpts         ClientOptions
	metricsRegistry *metrics.Registry
	contentCache    *cache.PersistentCache
	beforeFlush     []RepositoryWriterCallback

	*refCountedCloser
}

// Metrics provides access to the metrics registry.
func (r *immutableServerRepositoryParameters) Metrics() *metrics.Registry {
	return r.metricsRegistry
}

func (r *immutableServerRepositoryParameters) ClientOptions() ClientOptions {
	return r.cliOpts
}
