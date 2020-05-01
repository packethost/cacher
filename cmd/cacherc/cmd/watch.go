// Copyright © 2018 packet.net

package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/spf13/cobra"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:     "watch",
	Short:   "Register to watch an id for any changes",
	Example: "cacherc watch 224ee6ab-ad62-4070-a900-ed816444cec0 cb76ae54-93e9-401c-a5b2-d455bb3800b1",
	Args: func(_ *cobra.Command, args []string) error {
		return verifyUUIDs(args)
	},
	Run: func(cmd *cobra.Command, args []string) {
		conn := connectGRPC(cmd.Flags().GetString("facility"))
		stdoutLock := sync.Mutex{}
		for _, id := range args {
			go func(id string) {
				stream, err := conn.Watch(context.Background(), &cacher.GetRequest{ID: id})
				if err != nil {
					log.Fatal(err)
				}

				var hw *cacher.Hardware
				for hw, err = stream.Recv(); err == nil && hw != nil; hw, err = stream.Recv() {
					stdoutLock.Lock()
					fmt.Println(hw.JSON)
					stdoutLock.Unlock()
				}
				if err != nil && err != io.EOF {
					log.Fatal(err)
				}
			}(id)
		}
		select {}
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watchCmd.Flags().String("id", "", "id of the hardware")
}
