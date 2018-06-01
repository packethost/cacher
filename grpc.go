package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
	"github.com/pkg/errors"
)

const (
	clientPort = ":42111"
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
		err = errors.Wrap(err, "unmarshal json")
		sugar.Error(err)
		return &cacher.Empty{}, err
	}

	if h.ID == "" {
		err = errors.New("id must be set to a UUID")
		sugar.Error(err)
		return &cacher.Empty{}, err
	}

	var fn func() error
	msg := ""
	if h.State != "deleted" {
		msg = ("inserting into DB")
		fn = func() error { return insertIntoDB(ctx, s.db, in.Data) }
	} else {
		msg = ("deleting from DB")
		fn = func() error { return deleteFromDB(ctx, s.db, h.ID) }
	}

	sugar.Info(msg)
	err = fn()
	sugar.Info("done " + msg)
	if err != nil {
		sugar.Error(err)
	}

	return &cacher.Empty{}, err
}

// ByMAC implements cacher.CacherServer
func (s *server) ByMAC(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	s.mu.RLock()
	ready := s.dbReady
	s.mu.RUnlock()
	if !ready {
		return &cacher.Hardware{}, errors.New("DB is not ready")
	}

	return &cacher.Hardware{}, nil
}
