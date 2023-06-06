package cmd

import (
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/super-flat/parti/discovery"
	partilog "github.com/super-flat/parti/log"
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

			members := discovery.NewKubernetes(*namespace, podLabels, *portName, partilog.DefaultLogger)
			outChan, err := members.Listen(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}
			for change := range outChan {
				switch change.Type {
				case discovery.MemberAdded:
					log.Printf("member added %s @ %s:%d\n", change.ID, change.Host, change.Port)
				case discovery.MemberRemoved:
					log.Printf("member removed %s\n", change.ID)
				case discovery.MemberPinged:
					log.Printf("member pinged %s\n", change.ID)
				default:
					log.Printf("unhandled change %v", change.Type)
				}
			}
		},
	}

	namespace = cmd.Flags().String("namespace", "default", "")
	portName = cmd.Flags().String("port", "parti", "")
	flagLabels = cmd.Flags().StringArray("label", []string{"app:parti"}, "")

	rootCmd.AddCommand(cmd)
}
