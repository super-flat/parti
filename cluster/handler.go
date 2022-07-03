package cluster

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
)

type Handler interface {
	// Handle handles a message locally
	Handle(ctx context.Context, partitionID uint32, msg *anypb.Any) (*anypb.Any, error)
	// StartPartition blocks until a partition is started on this node
	StartPartition(ctx context.Context, partitionID uint32) error
	// ShutdownPartition blocks until a partition is fully shut down on this node
	ShutdownPartition(ctx context.Context, partitionID uint32) error
}
