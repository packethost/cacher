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
	"strconv"
	"strings"
	"syscall"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/packethost/pkg/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
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

func ingestFacility(ctx context.Context, client *packngo.Client, db *sql.DB, api, facility string) {
	label := prometheus.Labels{}
	var errCount int
	for errCount = 0; errCount < getMaxErrs(); errCount++ {
		logger.Info("starting fetch")
		label["op"] = "fetch"
		ingestCount.With(label).Inc()
		timer := prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(label).Set))
		data, err := fetchFacility(ctx, client, api, facility)
		if err != nil {
			ingestErrors.With(label).Inc()
			logger.With("error", err).Info()

			if ctx.Err() == context.Canceled {
				return
			}

			time.Sleep(5 * time.Second)
			continue
		}
		timer.ObserveDuration()
		logger.Info("done fetching")

		logger.Info("copying")
		label["op"] = "copy"
		timer = prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(label).Set))
		if err = copyin(ctx, db, data); err != nil {
			ingestErrors.With(label).Inc()

			l := logger.With("error", err)
			if pqErr := pqError(err); pqErr != nil {
				l = l.With("detail", pqErr.Detail, "where", pqErr.Where)
			}
			l.Info()

			if ctx.Err() == context.Canceled {
				return
			}

			time.Sleep(5 * time.Second)
			continue
		}
		timer.ObserveDuration()
		logger.Info("done copying")
		break
	}
	if errCount >= getMaxErrs() {
		err := errors.New("maximum fetch/copy errors reached")
		logger.Error(err)
		panic(err)
	}
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

func setupGRPC(ctx context.Context, client *packngo.Client, db *sql.DB, facility string, errCh chan<- error) ([]byte, time.Time) {
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

	var modT time.Time
	if stat, err := certFile.Stat(); err != nil {
		err = errors.Wrap(err, "failed to stat TLS cert")
		logger.Error(err)
		panic(err)
	} else {
		modT = stat.ModTime()
	}

	certPEM, err := ioutil.ReadAll(certFile)
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

	s := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)

	cacher.RegisterCacherServer(s, &server{
		packet: client,
		db:     db,
		quit:   ctx.Done(),
		watch:  map[string]chan string{},
		ingest: func() {
			ingestFacility(ctx, client, db, api, facility)
		},
	})
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

	return certPEM, modT
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(gitRevJSON)
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
	certPEM, modT := setupGRPC(ctx, client, db, facility, errCh)
	setupHTTP(ctx, certPEM, modT, errCh)

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
