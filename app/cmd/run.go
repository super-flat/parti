package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/super-flat/raft-poc/app/server"
)

func init() {
	rootCmd.AddCommand(runCMD)
}

var runCMD = &cobra.Command{
	Use:   "run",
	Short: "run the server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := server.NewConfigFromEnv()
		if err != nil {
			log.Fatalf("failed to create server config, %s", err.Error())
		}
		server.Run(cfg)
	},
}
