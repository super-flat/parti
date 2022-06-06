package raft

import (
	"context"

	"github.com/pkg/errors"
	partipb "github.com/super-flat/parti/gen/parti"
	grpcclient "github.com/super-flat/parti/grpc/client"
)

func ApplyOnLeader(node *Node, payload []byte) (interface{}, error) {
	// create the go context
	ctx := context.Background()
	// get the leader address
	serverAddr, _ := node.Raft.LeaderWithID()
	// check whether the server address is set or not
	if string(serverAddr) == "" {
		return nil, errors.New("unknown leader")
	}
	// create a gRPC client connection
	conn, err := grpcclient.GetClientConn(ctx, string(serverAddr))
	// handle the error
	if err != nil {
		return nil, err
	}
	// defer the connection close
	defer conn.Close()
	// create the raft client
	client := partipb.NewRaftServiceClient(conn)
	// apply log
	response, err := client.ApplyLog(ctx, &partipb.ApplyLogRequest{Request: payload})
	// handle the error
	if err != nil {
		return nil, err
	}

	// deserialize the response
	result, err := node.Serializer.Deserialize(response.Response)
	// handle the error
	if err != nil {
		return nil, err
	}
	// return the result
	return result, nil
}

func GetPeerDetails(address string) (*partipb.GetDetailsResponse, error) {
	// create the go context
	ctx := context.Background()
	// create a gRPC client connection
	conn, err := grpcclient.GetClientConn(ctx, address)
	// handle the error
	if err != nil {
		return nil, err
	}
	// defer the connection close
	defer conn.Close()
	// create the raft client
	client := partipb.NewRaftServiceClient(conn)
	// get details
	response, err := client.GetDetails(ctx, &partipb.GetDetailsRequest{})
	// handle the error
	if err != nil {
		return nil, err
	}
	// return the response
	return response, nil
}
