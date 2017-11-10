package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"
)

func query(args ...string) string {
	if len(args) == 0 {
		return ""
	}
	return "?" + strings.Join(args, "&")
}

func get(v interface{}, uri ...string) error {
	url := api + strings.Join(uri, "")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Close = true
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-AUTH-TOKEN", os.Getenv("PACKET_AUTH_TOKEN"))
	req.Header.Add("X-CONSUMER-TOKEN", os.Getenv("PACKET_CONSUMER_TOKEN"))
	req.Header.Add("X-PACKET-STAFF", "true")

	resp := httpCachedResponse(req)
	if resp == nil {
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		httpCacheResponse(req, resp)
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	return dec.Decode(&v)
}

// httpCacheKey returns the cache key for req.
func httpCacheKey(req *http.Request) string {
	return req.Method + " " + req.URL.String()
}

func httpCachedResponse(req *http.Request) *http.Response {
	key := httpCacheKey(req)
	d, err := cache.Get(key)
	if err != nil {
		return nil
	}

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(d)), req)
	if err != nil {
		cache.Delete(key)
		return nil
	}

	return resp
}

func httpCacheResponse(req *http.Request, resp *http.Response) {
	respBytes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return
	}

	cache.Set(httpCacheKey(req), respBytes, 5*time.Minute)
}
