// Copyright © 2018 packet.net

package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/spf13/cobra"
)

// ingestCmd represents the ingest command.
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
}
