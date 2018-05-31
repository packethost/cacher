package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/packethost/packngo"
	"go.uber.org/zap"
)

var (
	api   = "https://api.packet.net/"
	sugar *zap.SugaredLogger
)

func getMaxErrs() int {
	sMaxErrs := os.Getenv("CACHER_MAX_ERRS")
	if sMaxErrs == "" {
		sMaxErrs = "5"
	}

	max, err := strconv.Atoi(sMaxErrs)
	if err != nil {
		panic("unable to convert CACHER_MAX_ERRS to int")
	}
	return max
}

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	sugar = log.Sugar()
	defer log.Sync()

	if url := os.Getenv("PACKET_API_URL"); url != "" && url != api {
		api = url
		if !strings.HasSuffix(api, "/") {
			api += "/"
		}
	}

	client := packngo.NewClientWithAuth(os.Getenv("PACKET_CONSUMER_TOKEN"), os.Getenv("PACKET_API_AUTH_TOKEN"), nil)

	sugar.Infow("starting fetch")
	data, err := fetchFacility(client, api, os.Getenv("FACILITY"))
	sugar.Info("done fetching")
	if err != nil {
		sugar.Info(err)
	}

	fmt.Println(data)
	select {}
}
