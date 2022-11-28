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
	statsCMD.Flags().String("addr", "50101", "grpc address")
	rootCmd.AddCommand(statsCMD)
}

var statsCMD = &cobra.Command{
	Use:   "stats",
	Short: "get node stats",
	Run: func(cmd *cobra.Command, args []string) {
		grpcAddr, _ := cmd.Flags().GetString("addr")
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
		resp, err := client.Stats(context.Background(), &partipb.StatsRequest{})
		if err != nil {
			panic(err)
		}
		fmt.Println(protojson.Format(resp))
	},
}
