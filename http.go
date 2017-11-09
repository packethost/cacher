package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

const api = "https://api.packet.net/"

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

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	return dec.Decode(&v)
}
