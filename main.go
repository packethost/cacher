package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/packethost/pkg/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	api            = "https://api.packet.net/"
	gitRev         = "unknown"
	gitRevJSON     []byte
	logger         log.Logger
	grpcListenAddr = ":42111"
	httpListenAddr = ":42112"
	StartTime      = time.Now()
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

func connectDB() *sql.DB {
	db, err := sql.Open("postgres", "")
	if err != nil {
		logger.Error(err)
		panic(err)
	}
	if err := truncate(db); err != nil {
		if pqErr := pqError(err); pqErr != nil {
			logger.With("detail", pqErr.Detail, "where", pqErr.Where).Error(err)
		}
		panic(err)
	}
	return db
}

func setupGRPC(ctx context.Context, client *packngo.Client, db *sql.DB, facility string, errCh chan<- error) *server {
	var (
		certPEM []byte
		modT    time.Time
	)

	params := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	}

	if cert := os.Getenv("CACHER_TLS_CERT"); cert != "" {
		certPEM = []byte(cert)
		modT = time.Now()
	} else {
		certsDir := os.Getenv("CACHER_CERTS_DIR")
		if certsDir == "" {
			certsDir = "/certs/" + facility
		}
		if !strings.HasSuffix(certsDir, "/") {
			certsDir += "/"
		}

		certFile, err := os.Open(certsDir + "bundle.pem")
		if err != nil {
			err = errors.Wrap(err, "failed to open TLS cert")
			logger.Error(err)
			panic(err)
		}

		if stat, err := certFile.Stat(); err != nil {
			err = errors.Wrap(err, "failed to stat TLS cert")
			logger.Error(err)
			panic(err)
		} else {
			modT = stat.ModTime()
		}

		certPEM, err = ioutil.ReadAll(certFile)
		if err != nil {
			err = errors.Wrap(err, "failed to read TLS cert")
			logger.Error(err)
			panic(err)
		}
		keyPEM, err := ioutil.ReadFile(certsDir + "server-key.pem")
		if err != nil {
			err = errors.Wrap(err, "failed to read TLS key")
			logger.Error(err)
			panic(err)
		}

		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			err = errors.Wrap(err, "failed to ingest TLS files")
			logger.Error(err)
			panic(err)
		}

		params = append(params, grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	}

	s := grpc.NewServer(params...)
	server := &server{
		cert:   certPEM,
		modT:   modT,
		packet: client,
		db:     db,
		quit:   ctx.Done(),
		watch:  map[string]chan string{},
	}
	server.ingest = func() error {
		return server.ingestFacility(ctx, api, facility)
	}

	cacher.RegisterCacherServer(s, server)
	grpc_prometheus.Register(s)

	go func() {
		logger.Info("serving grpc")
		lis, err := net.Listen("tcp", grpcListenAddr)
		if err != nil {
			err = errors.Wrap(err, "failed to listen")
			logger.Error(err)
			panic(err)
		}

		errCh <- s.Serve(lis)
	}()

	go func() {
		<-ctx.Done()
		s.GracefulStop()
	}()

	return server
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(gitRevJSON)
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	res := struct {
		GitRev     string  `json:"git_rev"`
		Uptime     float64 `json:"uptime"`
		Goroutines int     `json:"goroutines"`
	}{
		GitRev:     gitRev,
		Uptime:     time.Since(StartTime).Seconds(),
		Goroutines: runtime.NumGoroutine(),
	}

	b, err := json.Marshal(&res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func setupGitRevJSON() {
	res := struct {
		GitRev  string `json:"git_rev"`
		Service string `json:"service_name"`
	}{
		GitRev:  gitRev,
		Service: "cacher",
	}
	b, err := json.Marshal(&res)
	if err != nil {
		err = errors.Wrap(err, "could not marshal version json")
		logger.Error(err)
		panic(err)
	}
	gitRevJSON = b
}

func setupHTTP(ctx context.Context, certPEM []byte, modTime time.Time, errCh chan<- error) *http.Server {
	http.HandleFunc("/cert", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "server.pem", modTime, bytes.NewReader(certPEM))
	})
	http.Handle("/metrics", promhttp.Handler())
	setupGitRevJSON()
	http.HandleFunc("/version", versionHandler)
	http.HandleFunc("/_packet/healthcheck", healthCheckHandler)
	srv := &http.Server{
		Addr: httpListenAddr,
	}
	go func() {
		logger.Info("serving http")
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	return srv
}

func main() {
	log, cleanup, err := log.Init("github.com/packethost/cacher")
	if err != nil {
		panic(err)
	}
	logger = log
	defer cleanup()

	if url := os.Getenv("PACKET_API_URL"); url != "" && url != api {
		api = url
		if !strings.HasSuffix(api, "/") {
			api += "/"
		}
	}

	client := packngo.NewClientWithAuth(os.Getenv("PACKET_CONSUMER_TOKEN"), os.Getenv("PACKET_API_AUTH_TOKEN"), nil)
	db := connectDB()
	facility := os.Getenv("FACILITY")
	setupMetrics(facility)

	if bindPort, ok := os.LookupEnv("NOMAD_PORT_internal_http"); ok {
		httpListenAddr = ":" + bindPort
	}

	if bindPort, ok := os.LookupEnv("NOMAD_PORT_internal_grpc"); ok {
		grpcListenAddr = ":" + bindPort
	}

	ctx, closer := context.WithCancel(context.Background())
	errCh := make(chan error, 2)
	server := setupGRPC(ctx, client, db, facility, errCh)
	setupHTTP(ctx, server.Cert(), server.ModTime(), errCh)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		logger.Error(err)
		panic(err)
	case sig := <-sigs:
		logger.With("signal", sig.String()).Info("signal received, stopping servers")
	}
	closer()

	// wait for both grpc and http servers to shutdown
	err = <-errCh
	if err != nil {
		logger.Error(err)
		panic(err)
	}
	err = <-errCh
	if err != nil {
		logger.Error(err)
		panic(err)
	}
}
