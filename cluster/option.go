package cluster

import (
	"github.com/super-flat/parti/logging"
)

// Option is the interface that applies a configuration option.
type Option interface {
	// Apply sets the Option value of a config.
	Apply(config *Cluster)
}

var _ Option = OptionFunc(nil)

// OptionFunc implements the Option interface.
type OptionFunc func(cluster *Cluster)

func (f OptionFunc) Apply(c *Cluster) {
	f(c)
}

// WithLogger sets the logger
func WithLogger(logger logging.Logger) Option {
	return OptionFunc(func(cluster *Cluster) {
		cluster.logger = logger
	})
}

// WithLogLevel sets the log level
func WithLogLevel(level logging.Level) Option {
	return OptionFunc(func(cluster *Cluster) {
		cluster.logLevel = level
	})
}

// WithPartitionCount sets the partition count
func WithPartitionCount(partitionCount uint32) Option {
	return OptionFunc(func(cluster *Cluster) {
		cluster.partitionCount = partitionCount
	})
}
