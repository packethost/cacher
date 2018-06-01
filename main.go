package main

import (
	"database/sql"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

func ingestFacility(client *packngo.Client, db *sql.DB, api, facility string) {
	var errCount int
	for errCount = 0; errCount < getMaxErrs(); errCount++ {
		sugar.Infow("starting fetch")
		data, err := fetchFacility(client, api, facility)
		sugar.Info("done fetching")
		if err != nil {
			sugar.Info(err)
			time.Sleep(5 * time.Second)
			continue
		}

		sugar.Info("copying")
		if err = copyin(db, data); err != nil {
			sugar.Info(err)
			time.Sleep(5 * time.Second)
			continue
		}
		sugar.Info("done copying")
		break
	}
	if errCount >= getMaxErrs() {
		sugar.Fatal("maximum fetch/copy errors reached")
	}
}

func connectDB() *sql.DB {
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
	return db
}

func setupGRPC(client *packngo.Client, db *sql.DB, facility string) *grpc.Server {
	tc, err := credentials.NewServerTLSFromFile("/certs/"+facility+"/server.pem", "/certs/"+facility+"/server-key.pem")
	if err != nil {
		sugar.Fatalf("failed to read TLS files: %v", err)
	}
	s := grpc.NewServer(grpc.Creds(tc),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)
	cacher.RegisterCacherServer(s, &server{
		packet: client,
		db:     db,
		ingest: func() {
			ingestFacility(client, db, api, facility)
		},
	})
	grpc_prometheus.Register(s)
	return s
}

func setupPromHTTP() {
	http.Handle("/metrics", promhttp.Handler())
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
	db := connectDB()
	facility := os.Getenv("FACILITY")
	s := setupGRPC(client, db, facility)
	setupPromHTTP()

	lis, err := net.Listen("tcp", clientPort)
	if err != nil {
		sugar.Fatalf("failed to listen: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs)
	go func() {
		sig := <-sigs
		sugar.Infow("signal received, stopping server", "signal", sig.String())
		s.GracefulStop()
	}()

	sugar.Info("serving grpc")
	if err := s.Serve(lis); err != nil {
		sugar.Fatalf("failed to serve: %v", err)
	}
}
