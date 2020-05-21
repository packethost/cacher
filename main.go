package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/packethost/cacher/hardware"
	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/packethost/pkg/env"
	"github.com/packethost/pkg/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	api        = mustParseURL("https://api.packet.net/")
	gitRev     = "unknown"
	gitRevJSON []byte
	logger     log.Logger
	StartTime  = time.Now()
)

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func setupGRPC(ctx context.Context, client *packngo.Client, facility string, errCh chan<- error) *server {

	params := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	}

	var certPEM []byte
	if cert := env.Get("CACHER_TLS_CERT"); cert != "" {
		certPEM = []byte(cert)
	} else {
		cert = env.Get("GRPC_CERT")
		if cert == "" {
			err := errors.New("GRPC_CERT missing from environment")
			logger.Fatal(err)
			panic(err)
		}
		certPEM = []byte(cert)

		key := env.Get("GRPC_KEY")
		if key == "" {
			err := errors.New("GRPC_KEY missing from environment")
			logger.Fatal(err)
			panic(err)
		}

		kp, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			err = errors.Wrap(err, "failed to ingest TLS files")
			logger.Error(err)
			panic(err)
		}

		params = append(params, grpc.Creds(credentials.NewServerTLSFromCert(&kp)))
	}

	s := grpc.NewServer(params...)
	server := &server{
		cert:   certPEM,
		modT:   StartTime,
		packet: client,
		quit:   ctx.Done(),
		hw:     hardware.New(hardware.Gauge(cacheCountTotal), hardware.Logger(logger.Package("hardware"))),
		watch:  map[string]chan string{},
	}

	cacher.RegisterCacherServer(s, server)
	grpc_prometheus.Register(s)

	go func() {
		logger.Info("serving grpc")
		lis, err := net.Listen("tcp", ":"+env.Get("GRPC_PORT", "42111"))
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
		Addr: ":" + env.Get("HTTP_PORT", "42112"),
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
	log, err := log.Init("github.com/packethost/cacher")
	if err != nil {
		panic(err)
	}
	logger = log
	defer logger.Close()

	if url := os.Getenv("PACKET_API_URL"); url != "" && mustParseURL(url).String() != api.String() {
		api = mustParseURL(url)
	}

	client := packngo.NewClientWithAuth(os.Getenv("PACKET_CONSUMER_TOKEN"), os.Getenv("PACKET_API_AUTH_TOKEN"), nil)
	facility := os.Getenv("FACILITY")
	setupMetrics(facility)

	ctx, closer := context.WithCancel(context.Background())
	errCh := make(chan error, 2)
	server := setupGRPC(ctx, client, facility, errCh)
	setupHTTP(ctx, server.Cert(), server.ModTime(), errCh)

	if err := server.ingest(ctx, api, facility); err != nil {
		logger.Error(err)
		panic(err)
	}

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
