package main

import (
	"context"
	"sync"
	"time"

	"github.com/packethost/cacher/hardware"
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
	quit   <-chan struct{}

	hw *hardware.Hardware

	ingestReadyLock sync.RWMutex
	ingestDone      bool

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

	cacheTotals.With(labels).Inc()
	timer := prometheus.NewTimer(cacheDuration.With(labels))

	id, err := s.hw.Add(in.Data)
	if err != nil {
		cacheErrors.With(labels).Inc()
		logger.Error(err)
		return nil, err
	}

	timer.ObserveDuration()

	s.watchLock.RLock()
	if ch := s.watch[id]; ch != nil {
		select {
		case ch <- in.Data:
		default:
			watchMissTotal.Inc()
			logger.With("id", id).Info("skipping blocked watcher")
		}
	}
	s.watchLock.RUnlock()

	return &cacher.Empty{}, err
}

// Ingest implements cacher.CacherServer
func (s *server) Ingest(ctx context.Context, in *cacher.Empty) (*cacher.Empty, error) {
	logger.Info("ingest")
	labels := prometheus.Labels{"method": "Ingest", "op": ""}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	logger.Info("Ingest called but is deprecated")

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
		s.ingestReadyLock.RLock()
		ready := s.ingestDone
		s.ingestReadyLock.RUnlock()
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
		return s.hw.ByMAC(in.MAC)
	})
}

// ByIP implements cacher.CacherServer
func (s *server) ByIP(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("ByIP", func() (string, error) {
		return s.hw.ByIP(in.IP)
	})
}

// ByID implements cacher.CacherServer
func (s *server) ByID(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return s.by("ByID", func() (string, error) {
		return s.hw.ByID(in.ID)
	})
}

// ALL implements cacher.CacherServer
func (s *server) All(_ *cacher.Empty, stream cacher.Cacher_AllServer) error {
	labels := prometheus.Labels{"method": "All", "op": "get"}

	cacheTotals.With(labels).Inc()
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	s.ingestReadyLock.RLock()
	ready := s.ingestDone
	s.ingestReadyLock.RUnlock()
	if !ready {
		cacheStalls.With(labels).Inc()
		return errors.New("DB is not ready")
	}

	timer := prometheus.NewTimer(cacheDuration.With(labels))
	defer timer.ObserveDuration()
	err := s.hw.All(func(j string) error {
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

	labels := prometheus.Labels{"method": "Watch", "op": "watch"}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	defer func() {
		s.watchLock.Lock()
		// Only delete and close if the existing channel matches
		if s.watch[in.ID] == ch {
			delete(s.watch, in.ID)
			close(ch)
		}
		s.watchLock.Unlock()
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
