package main

import (
	"context"
	"database/sql"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/packngo"
)

const (
	clientPort = ":42111"
)

type server struct {
	packet *packngo.Client
	db     *sql.DB
}

//go:generate protoc -I protos/cacher protos/cacher/cacher.proto --go_out=plugins=grpc:protos/cacher

// Push implements cacher.CacherServer
func (s *server) Push(ctx context.Context, in *cacher.PushRequest) (*cacher.Empty, error) {
	return &cacher.Empty{}, nil
}

// ByMAC implements cacher.CacherServer
func (s *server) ByMAC(ctx context.Context, in *cacher.GetRequest) (*cacher.Hardware, error) {
	return &cacher.Hardware{}, nil
}
