package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
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

	connStr := strings.Join([]string{
		"dbname=" + os.Getenv("POSTGRES_DB"),
		"host=" + os.Getenv("POSTGRES_HOST"),
		"password=" + os.Getenv("POSTGRES_PASSWORD"),
		"sslmode=disable",
		"user=" + os.Getenv("POSTGRES_USER"),
	}, " ")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		sugar.Fatal(err)
	}
	if err := truncate(db); err != nil {
		panic(err)
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
