package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/super-flat/raft-poc/gen/localpb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	pingCMD.Flags().String("addr", "", "grpc address")
	pingCMD.Flags().Uint32("partition", 0, "partitiont to ping")

	rootCmd.AddCommand(pingCMD)
}

var pingCMD = &cobra.Command{
	Use:   "ping",
	Short: "ping a partition",
	Run: func(cmd *cobra.Command, args []string) {
		grpcAddr, _ := cmd.Flags().GetString("addr")
		partition, _ := cmd.Flags().GetUint32("partition")

		conn, err := grpc.Dial(
			grpcAddr,
			grpc.WithInsecure(),
			grpc.WithBlock(),
			grpc.EmptyDialOption{},
		)
		if err != nil {
			// todo: don't panic here
			panic(err)
		}
		client := localpb.NewClusteringClient(conn)

		req := &localpb.PingRequest{PartitionId: partition}
		resp, err := client.Ping(context.Background(), req)
		if err != nil {
			panic(err)
		}
		fmt.Println(protojson.Format(resp))
	},
}
