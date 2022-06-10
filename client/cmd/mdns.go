package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/super-flat/parti/node/raftwrapper/discovery"
)

func init() {
	mdnsCMD.Flags().Int("port", 0, "grpc port")
	rootCmd.AddCommand(mdnsCMD)
}

var mdnsCMD = &cobra.Command{
	Use:   "mdns",
	Short: "mdns test",
	Run: func(cmd *cobra.Command, args []string) {
		grpcPort, err := cmd.Flags().GetInt("port")
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("starting on port %d", grpcPort)

		disco := discovery.NewMDNSDiscovery(grpcPort)

		nodesChan, err := disco.Start()
		if err != nil {
			log.Fatal(err)
		}
		log.Print("started")
		defer disco.Stop()
		for node := range nodesChan {
			log.Printf("found node %s", node)
		}
	},
}
