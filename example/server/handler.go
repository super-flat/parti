package server

import (
	"context"
	"fmt"

	"github.com/super-flat/parti/cluster"
	"github.com/super-flat/parti/log"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type ExampleHandler struct {
	logger log.Logger
}

var _ cluster.Handler = &ExampleHandler{}

func (e *ExampleHandler) Handle(ctx context.Context, partitionID uint32, msg *anypb.Any) (*anypb.Any, error) {
	// unpack inner msg
	innerMsg, err := msg.UnmarshalNew()
	if err != nil {
		return nil, err
	}

	resp := ""

	switch v := innerMsg.(type) {
	case *wrapperspb.StringValue:
		resp = fmt.Sprintf("replying to '%s'", v.GetValue())
	default:
		resp = fmt.Sprintf("responding, partition=%d, type=%s", partitionID, msg.GetTypeUrl())
	}
	return anypb.New(wrapperspb.String(resp))
}

func (e *ExampleHandler) StartPartition(ctx context.Context, partitionID uint32) error {
	e.logger.Info("starting partition", partitionID)
	return nil
}

func (e *ExampleHandler) ShutdownPartition(ctx context.Context, partitionID uint32) error {
	e.logger.Infof("shutting down partition: %d", partitionID)
	// time.Sleep(time.Second * 10)
	return nil
}
