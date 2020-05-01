// Copyright © 2018 packet.net

package cmd

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/spf13/cobra"
)

// allCmd represents the all command
var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Get all known hardware for facility",
	Run: func(cmd *cobra.Command, args []string) {
		conn := connectGRPC(cmd.Flags().GetString("facility"))
		alls, err := conn.All(context.Background(), &cacher.Empty{})
		if err != nil {
			log.Fatal(err)
		}

		var hw *cacher.Hardware
		for hw, err = alls.Recv(); err == nil && hw != nil; hw, err = alls.Recv() {
			fmt.Println(hw.JSON)
		}
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(allCmd)
}
