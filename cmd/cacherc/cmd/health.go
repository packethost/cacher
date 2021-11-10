// Copyright © 2021 packet.net

package cmd

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	health "google.golang.org/grpc/health/grpc_health_v1"
)

var healthWatch bool

// healthCmd represents the health command.
var healthCmd = &cobra.Command{
	Use:     "health",
	Short:   "Check cacher health",
	Example: "cacherc -f $fac health",
	Run: func(cmd *cobra.Command, args []string) {
		conn := connectGRPC(cmd.Flags().GetString("facility"))
		h := health.NewHealthClient(conn.Conn)
		if !healthWatch {
			resp, err := h.Check(context.Background(), &health.HealthCheckRequest{})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(resp.String())
			return
		}

		w, err := h.Watch(context.Background(), &health.HealthCheckRequest{})
		if err != nil {
			log.Fatal(err)
		}

		var resp health.HealthCheckResponse
		for err = w.RecvMsg(&resp); err == nil; err = w.RecvMsg(&resp) {
			fmt.Println(resp.String())
		}
		if err != nil && !errors.Is(err, io.EOF) {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.PersistentFlags().BoolVarP(&healthWatch, "watch", "w", false, "continually watch for status updates")
}
