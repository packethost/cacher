// Copyright © 2018 packet.net

package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/spf13/cobra"
)

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Trigger cacher to ingest",
	Long:  "This command only signals cacher to ingest if it has not already done so.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("ingest called")

		conn := connectGRPC(cmd.Flags().GetString("facility"))
		_, err := conn.Ingest(context.Background(), &cacher.Empty{})
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// ingestCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// ingestCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
