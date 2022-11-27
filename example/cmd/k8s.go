package cmd

import (
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/super-flat/parti/cluster/membership"
)

func init() {

	var flagLabels *[]string
	var namespace *string
	var portName *string

	cmd := &cobra.Command{
		Use:   "k",
		Short: "test",
		Run: func(cmd *cobra.Command, args []string) {

			podLabels := make(map[string]string, len(*flagLabels))
			for _, pair := range *flagLabels {
				pairValues := strings.Split(pair, ":")
				if len(pairValues) != 2 {
					log.Fatalf("malformed label %s", pair)
				}
				labelKey := strings.TrimSpace(pairValues[0])
				labelValue := strings.TrimSpace(pairValues[1])
				podLabels[labelKey] = labelValue
			}

			members := membership.NewKubernetes(*namespace, podLabels, *portName)
			outChan, err := members.Listen(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}
			for change := range outChan {
				log.Printf("change %s %d\n", change.ID, change.Change)
			}
		},
	}

	namespace = cmd.Flags().String("namespace", "default", "")
	portName = cmd.Flags().String("port", "raft", "")
	flagLabels = cmd.Flags().StringArray("label", []string{"app:parti"}, "")

	rootCmd.AddCommand(cmd)
}
