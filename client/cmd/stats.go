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
	statsCMD.Flags().String("addr", "", "grpc address")
	rootCmd.AddCommand(statsCMD)
}

var statsCMD = &cobra.Command{
	Use:   "stats",
	Short: "get node stats",
	Run: func(cmd *cobra.Command, args []string) {
		grpcAddr, _ := cmd.Flags().GetString("addr")
		conn, err := grpcclient.NewBuilder().
			WithInsecure().
			GetConn(cmd.Context(), grpcAddr)

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
