package cluster

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
)

type Handler interface {
	// Handle handles a message locally
	Handle(ctx context.Context, partitionID uint32, msg *anypb.Any) (*anypb.Any, error)
}
