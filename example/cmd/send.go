package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	sendCMD.Flags().String("addr", "", "grpc address")
	sendCMD.Flags().Uint32("partition", 0, "partitiont to send")

	rootCmd.AddCommand(sendCMD)
}

var sendCMD = &cobra.Command{
	Use:     "send",
	Short:   "send message to a partition",
	Example: "send --addr localhost:50001 --partition 9 hello world",
	Run: func(cmd *cobra.Command, args []string) {
		partition, _ := cmd.Flags().GetUint32("partition")
		target, _ := cmd.Flags().GetString("addr")
		if !strings.HasPrefix(target, "http://") {
			target = fmt.Sprintf("http://%s", target)
		}
		if strings.HasSuffix(target, "/") {
			target = strings.TrimRight(target, "/")
		}
		target = fmt.Sprintf("%s/send", target)
		req, _ := http.NewRequest("GET", target, nil)
		q := req.URL.Query()
		q.Add("partition", strconv.FormatUint(uint64(partition), 10))
		q.Add("message", strings.Join(args, " "))
		req.URL.RawQuery = q.Encode()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		respBytes, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(respBytes))
	},
}
