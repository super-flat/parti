package server

import (
	"context"
	"fmt"

	"github.com/super-flat/raft-poc/node"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type ExampleHandler struct{}

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

var _ node.Handler = &ExampleHandler{}
