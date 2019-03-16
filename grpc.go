package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	cert []byte
	modT time.Time

	packet *packngo.Client
	db     *sql.DB
	quit   <-chan struct{}

	once   sync.Once
	ingest func()

	dbLock  sync.RWMutex
	dbReady bool

	watchLock sync.RWMutex
	watch     map[string]chan string
}

//go:generate protoc -I protos/cacher protos/cacher/cacher.proto --go_out=plugins=grpc:protos/cacher

// Push implements cacher.CacherServer
func (s *server) Push(ctx context.Context, in *cacher.PushRequest) (*cacher.Empty, error) {
	logger.Info("push")
	labels := prometheus.Labels{"method": "Push", "op": ""}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	// must be a copy so deferred cacheInFlight.Dec matches the Inc
	labels = prometheus.Labels{"method": "Push", "op": ""}

	s.once.Do(func() {
		logger.Info("ingestion goroutine is starting")
		// in a goroutine to not block Push and possibly timeout
		go func() {
			logger.Info("ingestion is starting")
			s.ingest()
			s.dbLock.Lock()
			s.dbReady = true
			s.dbLock.Unlock()
			logger.Info("ingestion is done")
		}()
		logger.Info("ingestion goroutine is started")
	})

	var h struct {
		ID    string
		State string
	}
	err := json.Unmarshal([]byte(in.Data), &h)
	if err != nil {
		cacheTotals.With(labels).Inc()
		cacheErrors.With(labels).Inc()
		err = errors.Wrap(err, "unmarshal json")
		logger.Error(err)
		return &cacher.Empty{}, err
	}

	if h.ID == "" {
		cacheTotals.With(labels).Inc()
		cacheErrors.With(labels).Inc()
		err = errors.New("id must be set to a UUID")
		logger.Error(err)
		return &cacher.Empty{}, err
	}

	logger.With("id", h.ID).Info("data pushed")

	var fn func() error
	msg := ""
	if h.State != "deleted" {
		labels["op"] = "insert"
		msg = "inserting into DB"
		fn = func() error { return insertIntoDB(ctx, s.db, in.Data) }
	} else {
		msg = "deleting from DB"
		labels["op"] = "delete"
		fn = func() error { return deleteFromDB(ctx, s.db, h.ID) }
	}

	cacheTotals.With(labels).Inc()
	timer := prometheus.NewTimer(cacheDuration.With(labels))
	defer timer.ObserveDuration()

	logger.Info(msg)
	err = fn()
	logger.Info("done " + msg)
	if err != nil {
		cacheErrors.With(labels).Inc()
		l := logger
		if pqErr := pqError(err); pqErr != nil {
			l = l.With("detail", pqErr.Detail, "where", pqErr.Where)
		}
		l.Error(err)
	}

	s.watchLock.RLock()
	if ch := s.watch[h.ID]; ch != nil {
		select {
		case ch <- in.Data:
		default:
			watchMissTotal.Inc()
			logger.With("id", h.ID).Info("skipping blocked watcher")
		}
	}
	s.watchLock.RUnlock()

	return &cacher.Empty{}, err
}

func (s *server) ingestFacility(ctx context.Context, api, facility string) error {
	logger.Info("ingest")
	label := prometheus.Labels{"method": "Ingest", "op": ""}
	cacheInFlight.With(label).Inc()
	defer cacheInFlight.With(label).Dec()

	var errCount int
	for errCount = 0; errCount < getMaxErrs(); errCount++ {
		logger.Info("starting fetch")
		label["op"] = "fetch"
		ingestCount.With(label).Inc()
		timer := prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(label).Set))
		data, err := fetchFacility(ctx, s.packet, api, facility)
		if err != nil {
			ingestErrors.With(label).Inc()
			logger.With("error", err).Info()

			if ctx.Err() == context.Canceled {
				return nil
			}

			time.Sleep(5 * time.Second)
			continue
		}
		timer.ObserveDuration()
		logger.Info("done fetching")

		logger.Info("copying")
		label["op"] = "copy"
		ingestCount.With(label).Inc()
		timer = prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(label).Set))
		if err = copyin(ctx, s.db, data); err != nil {
			ingestErrors.With(label).Inc()

			l := logger.With("error", err)
			if pqErr := pqError(err); pqErr != nil {
				l = l.With("detail", pqErr.Detail, "where", pqErr.Where)
			}
			l.Info()

			if ctx.Err() == context.Canceled {
				return nil
			}

			time.Sleep(5 * time.Second)
			continue
		}
		timer.ObserveDuration()
		logger.Info("done copying")

		return nil
	}

	return errors.New("maximum fetch/copy errors reached")
}

// Ingest implements cacher.CacherServer
func (s *server) Ingest(ctx context.Context, in *cacher.Empty) (*cacher.Empty, error) {
	logger.Info("ingest")
	labels := prometheus.Labels{"method": "Ingest", "op": ""}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	s.once.Do(func() {
		logger.Info("ingestion is starting")
		s.ingest()
		s.dbLock.Lock()
		s.dbReady = true
		s.dbLock.Unlock()
		logger.Info("ingestion is done")
	})

	return &cacher.Empty{}, nil
}

func (s *server) by(method string, fn func() (string, error)) (*cacher.Hardware, error) {
	labels := prometheus.Labels{"method": method, "op": "get"}

	cacheTotals.With(labels).Inc()
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	timer := prometheus.NewTimer(cacheDuration.With(labels))
	defer timer.ObserveDuration()
	j, err := fn()
	if err != nil {
		cacheErrors.With(labels).Inc()
		return &cacher.Hardware{}, err
	}

	if j == "" {
		s.dbLock.RLock()
		ready := s.dbReady
		s.dbLock.RUnlock()
		if !ready {
			cacheStalls.With(labels).Inc()
			return &cacher.Hardware{}, errors.New("DB is not ready")
		}
	}

	cacheHits.With(labels).Inc()
	return &cacher.Hardware{JSON: j}, nil
}

// ByMAC implements cacher.CacherServer
func (s *server) ByMAC(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("ByMAC", func() (string, error) {
		return getByMAC(ctx, s.db, in.MAC)
	})
}

// ByIP implements cacher.CacherServer
func (s *server) ByIP(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("ByIP", func() (string, error) {
		return getByIP(ctx, s.db, in.IP)
	})
}

// ByID implements cacher.CacherServer
func (s *server) ByID(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("ByID", func() (string, error) {
		return getByID(ctx, s.db, in.ID)
	})
}

// ALL implements cacher.CacherServer
func (s *server) All(_ *cacher.Empty, stream cacher.Cacher_AllServer) error {
	labels := prometheus.Labels{"method": "All", "op": "get"}

	cacheTotals.With(labels).Inc()
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	s.dbLock.RLock()
	ready := s.dbReady
	s.dbLock.RUnlock()
	if !ready {
		cacheStalls.With(labels).Inc()
		return errors.New("DB is not ready")
	}

	timer := prometheus.NewTimer(cacheDuration.With(labels))
	defer timer.ObserveDuration()
	err := getAll(s.db, func(j string) error {
		return stream.Send(&cacher.Hardware{JSON: j})
	})
	if err != nil {
		cacheErrors.With(labels).Inc()
		return err
	}

	cacheHits.With(labels).Inc()
	return nil
}

// Watch implements cacher.CacherServer
func (s *server) Watch(in *cacher.GetRequest, stream cacher.Cacher_WatchServer) error {
	l := logger.With("id", in.ID)

	ch := make(chan string, 1)
	s.watchLock.Lock()
	old, ok := s.watch[in.ID]
	if ok {
		l.Info("evicting old watch")
		close(old)
	}
	s.watch[in.ID] = ch
	s.watchLock.Unlock()

	labels := prometheus.Labels{"method": "Watch", "op": "push"}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	disconnect := true
	defer func() {
		if !disconnect {
			return
		}
		s.watchLock.Lock()
		delete(s.watch, in.ID)
		s.watchLock.Unlock()
		close(ch)
	}()

	hw := &cacher.Hardware{}
	for {
		select {
		case <-s.quit:
			l.Info("server is shutting down")
			return status.Error(codes.OK, "server is shutting down")
		case <-stream.Context().Done():
			l.Info("client disconnected")
			return status.Error(codes.OK, "client disconnected")
		case j, ok := <-ch:
			if !ok {
				disconnect = false
				l.Info("we are being evicted, goodbye")
				// ch was replaced and already closed
				return status.Error(codes.Unknown, "evicted")
			}

			hw.Reset()
			hw.JSON = j
			err := stream.Send(hw)
			if err != nil {
				cacheErrors.With(labels).Inc()
				err = errors.Wrap(err, "stream send")
				l.Error(err)
				return err
			}
		}
	}
}

// Cert returns the public cert that can be served to clients
func (s *server) Cert() []byte {
	return s.cert
}

// ModTime returns the modified-time of the grpc cert
func (s *server) ModTime() time.Time {
	return s.modT
}
