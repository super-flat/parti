package cmd

import (
	"github.com/spf13/cobra"
	"github.com/super-flat/raft-poc/example/server"
)

func init() {
	rootCmd.AddCommand(serverCMD)
}

var serverCMD = &cobra.Command{
	Use:   "server",
	Short: "run the example server",
	Run: func(cmd *cobra.Command, args []string) {
		server.Run()
	},
}
