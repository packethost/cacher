package main

import (
	"encoding/json"
	"os"

	"github.com/garyburd/redigo/redis"
)

var conn redis.Conn

func connectCache() error {
	c, err := redis.Dial("tcp", os.Getenv("REDIS_MASTER"))
	if err != nil {
		return err
	}
	conn = c
	return nil
}

func setCache(f *facility) error {
	j, err := json.Marshal(f)
	if err != nil {
		return err
	}

	_, err = conn.Do("JSON.SET", "cacher."+f.Code, ".", j)
	return err
}
