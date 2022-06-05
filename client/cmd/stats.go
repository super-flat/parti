package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/super-flat/parti/gen/localpb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	statsCMD.Flags().String("addr", "", "grpc address")
	rootCmd.AddCommand(statsCMD)
}

var statsCMD = &cobra.Command{
	Use:   "stats",
	Short: "get node stats",
	Run: func(cmd *cobra.Command, args []string) {
		grpcAddr, _ := cmd.Flags().GetString("addr")
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
		resp, err := client.Stats(context.Background(), &localpb.StatsRequest{})
		if err != nil {
			panic(err)
		}
		fmt.Println(protojson.Format(resp))
	},
}
