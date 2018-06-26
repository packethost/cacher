// Copyright © 2018 packet.net

package cmd

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/spf13/cobra"
)

// ipCmd represents the ip command
var ipCmd = &cobra.Command{
	Use:     "ip",
	Short:   "Get hardware by any associated ip",
	Example: "cacherc ip 10.0.0.2 10.0.0.3",
	Args: func(_ *cobra.Command, args []string) error {
		for _, arg := range args {
			if net.ParseIP(arg) == nil {
				return fmt.Errorf("invalid ip: %s", arg)
			}
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		conn := connectGRPC(cmd.Flags().GetString("facility"))
		for _, ip := range args {
			hw, err := conn.ByIP(context.Background(), &cacher.GetRequest{IP: ip})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(hw.JSON)
		}
	},
}

func init() {
	rootCmd.AddCommand(ipCmd)
}
