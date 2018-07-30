package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"
)

func fetchFacility(ctx context.Context, client *packngo.Client, api, facility string) ([]map[string]interface{}, error) {
	var j []map[string]interface{}
	for page, lastPage := 1, 1; page <= lastPage; page++ {
		req, err := client.NewRequest("GET", api+"staff/cacher/hardware?facility="+facility+"&sort_by=created_at&sort_direction=asc&per_page=50&page="+strconv.Itoa(page), nil)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("NewRequest page=%d", page))
		}
		req = req.WithContext(ctx)
		req.Header.Add("X-Packet-Staff", "true")

		r := struct {
			Meta struct {
				CurrentPage int `json:"current_page"`
				LastPage    int `json:"last_page"`
				Total       int `json:"total"`
			}
			Hardware []map[string]interface{}
		}{}
		_, err = client.Do(req, &r)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("packngo: page=%d", page))
		}

		if j == nil {
			j = make([]map[string]interface{}, 0, r.Meta.Total)
		}

		j = append(j, r.Hardware...)
		lastPage = r.Meta.LastPage
		sugar.Infow("fetched a page", "have", len(j), "want", r.Meta.Total)
	}
	return j, nil
}
