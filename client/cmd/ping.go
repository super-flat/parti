package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	pingCMD.Flags().String("addr", "", "grpc address")
	pingCMD.Flags().Uint32("partition", 0, "partition to ping")

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
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.EmptyDialOption{},
		)
		if err != nil {
			// todo: don't panic here
			panic(err)
		}
		client := partipb.NewClusteringClient(conn)

		req := &partipb.PingRequest{PartitionId: partition}
		resp, err := client.Ping(context.Background(), req)
		if err != nil {
			panic(err)
		}
		fmt.Println(protojson.Format(resp))
	},
}
