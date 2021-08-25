package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/packethost/cacher/hardware"
	"github.com/packethost/packngo"
	"github.com/packethost/pkg/env"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func fetchFacilityPage(ctx context.Context, client *packngo.Client, u string) ([]map[string]interface{}, uint, error) {
	req, err := client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to create fetch request")
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
		return nil, 0, errors.Wrap(err, "failed to fetch page")
	}

	return r.Hardware, uint(r.Meta.Total), nil
}

func fetchFacility(ctx context.Context, client *packngo.Client, api *url.URL, facility string, data chan<- []map[string]interface{}) error {
	logger.Info("fetch start")

	labels := prometheus.Labels{"method": "Ingest", "op": "fetch"}

	ingestCount.With(labels).Inc()
	timer := prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(labels).Set))

	concurrentFetches := env.Int("CACHER_CONCURRENT_FETCHES", 4)
	pool := workerpool.New(concurrentFetches)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int("CACHER_CONCURRENT_FETCHES", concurrentFetches))

	defer close(data)

	api.Path = "/staff/cacher/hardware"
	// this query is used to fetch the first page then mutated later to paginate
	q := api.Query()
	q.Set("facility", facility)
	q.Set("sort_by", "created_at")
	q.Set("sort_direction", "asc")
	q.Set("per_page", "1")

	api.RawQuery = q.Encode()

	_, total, err := fetchFacilityPage(ctx, client, api.String())
	if err != nil {
		return errors.Wrap(err, "failed to fetch initial page")
	}

	perPage := env.Int("CACHER_FETCH_PER_PAGE", 50)

	if perPage > 1000 {
		logger.Info("limiting per_page to 1000")
		perPage = 1000
	}

	iterations := int(total) / perPage
	if int(total)%perPage != 0 {
		iterations++
	}

	span.SetAttributes(
		attribute.String("fetchFacility.path", api.Path),
		attribute.String("fetchFacility.base.query", q.Encode()),
		attribute.Int("CACHER_FETCH_PER_PAGE", perPage),
		attribute.Int("fetchFacility.paging.total", int(total)),
		attribute.Int("fetchFacility.paging.iterations", iterations),
	)

	q.Set("per_page", strconv.Itoa(perPage))
	tStart := time.Now()

	for i := 1; i <= iterations; i++ {
		q.Set("page", strconv.Itoa(i))
		api.RawQuery = q.Encode()
		u := api.String()
		page := i

		span.AddEvent("fetching page",
			trace.WithAttributes(attribute.Int("page", page)),
			trace.WithAttributes(attribute.String("query", api.RawQuery)),
		)

		pool.Submit(func() {
			logger.With("page", page).Info("fetching a page")
			tPageStart := time.Now()
			hw, _, err := fetchFacilityPage(ctx, client, u)
			if err != nil {
				logger.Fatal(errors.Wrapf(err, "failed to fetch page"))
				return
			}
			logger.With("page", page, "pages", iterations, "duration", time.Since(tPageStart)).Info("fetched a page")
			data <- hw
		})
	}

	pool.StopWait()

	timer.ObserveDuration()
	logger.With("duration", time.Since(tStart)).Info("fetch done")

	return nil
}

func copyin(hw *hardware.Hardware, data <-chan []map[string]interface{}) error {
	for hws := range data {
		if err := copyInEach(hw, hws); err != nil {
			return err
		}
	}

	return nil
}

func copyInEach(hw *hardware.Hardware, data []map[string]interface{}) error {
	logger.Info("copy start")
	labels := prometheus.Labels{"method": "Ingest", "op": "copy"}
	ingestCount.With(labels).Inc()
	timer := prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(labels).Set))

	now := time.Now()

	for _, j := range data {
		var q []byte

		q, err := json.Marshal(j)
		if err != nil {
			return errors.Wrap(err, "marshal json")
		}

		_, err = hw.Add(string(q))
		if err != nil {
			logger.With("json", string(q)).Error(err)
			return err
		}
	}

	timer.ObserveDuration()
	logger.With("duration", time.Since(now)).Info("copy done")

	return nil
}

func (s *server) ingest(ctx context.Context, api *url.URL, facility string) error {
	logger.Info("ingestion is starting")
	defer logger.Info("ingestion is done")
	cacherState.Set(1)

	labels := prometheus.Labels{"method": "Ingest", "op": ""}
	cacheInFlight.With(labels).Inc()

	defer cacheInFlight.With(labels).Dec()

	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup
	wg.Add(2)

	ch := make(chan []map[string]interface{}, 1)
	errCh := make(chan error, 1)
	tStart := time.Now()

	go func() {
		defer wg.Done()

		if err := fetchFacility(ctx, s.packet, api, facility, ch); err != nil {
			labels := prometheus.Labels{"method": "Ingest", "op": "fetch"}
			ingestErrors.With(labels).Inc()
			logger.Error(err)

			if errors.Is(ctx.Err(), context.Canceled) {
				return
			}

			cancel()
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()

		if err := copyin(s.hw, ch); err != nil {
			labels := prometheus.Labels{"method": "Ingest", "op": "copy"}
			ingestErrors.With(labels).Inc()

			// logging is already taken care of

			if errors.Is(ctx.Err(), context.Canceled) {
				return
			}

			cancel()
			errCh <- err
		}
	}()

	wg.Wait()
	logger.With("duration", time.Since(tStart)).Info("ingest done")
	cacherState.Set(2)
	cancel()

	select {
	case err := <-errCh:
		return err
	default:
	}

	s.ingestReadyLock.Lock()
	s.ingestDone = true
	s.ingestReadyLock.Unlock()

	return nil
}
