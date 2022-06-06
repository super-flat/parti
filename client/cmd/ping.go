package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	partipb "github.com/super-flat/parti/gen/parti"
	grpcclient "github.com/super-flat/parti/grpc/client"
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

		conn, err := grpcclient.NewBuilder().
			WithInsecure().
			GetConn(cmd.Context(), grpcAddr)

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
