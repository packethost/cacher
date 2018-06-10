package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type server struct {
	packet *packngo.Client
	db     *sql.DB

	once   sync.Once
	ingest func()

	mu      sync.RWMutex
	dbReady bool
}

//go:generate protoc -I protos/cacher protos/cacher/cacher.proto --go_out=plugins=grpc:protos/cacher

// Push implements cacher.CacherServer
func (s *server) Push(ctx context.Context, in *cacher.PushRequest) (*cacher.Empty, error) {
	sugar.Info("push")
	labels := prometheus.Labels{"method": "Push", "op": ""}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	// must be a copy so deferred cacheInFlight.Dec matches the Inc
	labels = prometheus.Labels{"method": "Push", "op": ""}

	s.once.Do(func() {
		sugar.Info("ingestion goroutine is starting")
		// in a goroutine to not block Push and possibly timeout
		go func() {
			sugar.Info("ingestion is starting")
			s.ingest()
			s.mu.Lock()
			s.dbReady = true
			s.mu.Unlock()
			sugar.Info("ingestion is done")
		}()
		sugar.Info("ingestion goroutine is started")
	})

	sugar.Info(in.Data)
	var h struct {
		ID    string
		State string
	}

	err := json.Unmarshal([]byte(in.Data), &h)
	if err != nil {
		cacheTotals.With(labels).Inc()
		cacheErrors.With(labels).Inc()
		err = errors.Wrap(err, "unmarshal json")
		sugar.Error(err)
		return &cacher.Empty{}, err
	}

	if h.ID == "" {
		cacheTotals.With(labels).Inc()
		cacheErrors.With(labels).Inc()
		err = errors.New("id must be set to a UUID")
		sugar.Error(err)
		return &cacher.Empty{}, err
	}

	var fn func() error
	msg := ""
	if h.State != "deleted" {
		labels["op"] = "insert"
		msg = ("inserting into DB")
		fn = func() error { return insertIntoDB(ctx, s.db, in.Data) }
	} else {
		msg = ("deleting from DB")
		labels["op"] = "delete"
		fn = func() error { return deleteFromDB(ctx, s.db, h.ID) }
	}

	cacheTotals.With(labels).Inc()
	timer := prometheus.NewTimer(cacheDuration.With(labels))
	defer timer.ObserveDuration()

	sugar.Info(msg)
	err = fn()
	sugar.Info("done " + msg)
	if err != nil {
		cacheErrors.With(labels).Inc()
		sugar.Error(err)
		if pqErr := pqError(err); pqErr != nil {
			sugar.Error(pqErr.Detail)
			sugar.Error(pqErr.Where)
		}
	}

	return &cacher.Empty{}, err
}

func (s *server) by(method string, fn func() (string, error)) (*cacher.Hardware, error) {
	labels := prometheus.Labels{"op": "get", "method": "By" + method}

	cacheTotals.With(labels).Inc()
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	s.mu.RLock()
	ready := s.dbReady
	s.mu.RUnlock()
	if !ready {
		cacheStalls.With(labels).Inc()
		return &cacher.Hardware{}, errors.New("DB is not ready")
	}

	timer := prometheus.NewTimer(cacheDuration.With(labels))
	defer timer.ObserveDuration()
	j, err := fn()
	if err != nil {
		cacheErrors.With(labels).Inc()
		return &cacher.Hardware{}, err
	}

	cacheHits.With(labels).Inc()
	return &cacher.Hardware{JSON: j}, nil
}

// ByMAC implements cacher.CacherServer
func (s *server) ByMAC(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("MAC", func() (string, error) {
		return getByMAC(ctx, s.db, in.MAC)
	})
}

// ByIP implements cacher.CacherServer
func (s *server) ByIP(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("IP", func() (string, error) {
		return getByIP(ctx, s.db, in.IP)
	})
}

// ByID implements cacher.CacherServer
func (s *server) ByID(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return &cacher.Hardware{}, nil
}

// ALL implements cacher.CacherServer
func (s *server) All(_ *cacher.Empty, stream cacher.Cacher_AllServer) error {
	labels := prometheus.Labels{"op": "get", "method": "All"}

	cacheTotals.With(labels).Inc()
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	s.mu.RLock()
	ready := s.dbReady
	s.mu.RUnlock()
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
