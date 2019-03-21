package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
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
		logger.With("have", len(j), "want", r.Meta.Total).Info("fetched a page")
	}
	return j, nil
}

func copyin(ctx context.Context, db *sql.DB, data []map[string]interface{}) error {
	now := time.Now()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return errors.Wrap(err, "BEGIN transaction")
	}

	stmt, err := tx.Prepare(`
	INSERT INTO
		hardware (data)
	VALUES
		($1)
	`)

	if err != nil {
		return errors.Wrap(err, "PREPARE INSERT")
	}

	for _, j := range data {
		var q []byte
		q, err = json.Marshal(j)
		if err != nil {
			return errors.Wrap(err, "marshal json")
		}
		_, err = stmt.Exec(q)
		if err != nil {
			return errors.Wrap(err, "INSERT")
		}
	}

	err = stmt.Close()
	if err != nil {
		return errors.Wrap(err, "Close")
	}

	// Remove duplicates, keeping what has already been inserted via insertIntoDB since startup
	_, err = tx.Exec(`
	DELETE FROM hardware a
	USING hardware b
	WHERE a.id IS NULL
	AND (a.data ->> 'id')::uuid = b.id
	`)
	if err != nil {
		return errors.Wrap(err, "delete overwrite")
	}

	_, err = tx.Exec(`
	UPDATE hardware
	SET (inserted_at, id) =
	  ($1::timestamptz, (data ->> 'id')::uuid);
	`, now)
	if err != nil {
		return errors.Wrap(err, "set inserted_at and id")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "COMMIT")
	}

	_, err = db.Exec("VACUUM FULL ANALYZE")
	if err != nil {
		return errors.Wrap(err, "VACCUM FULL ANALYZE")
	}

	return nil
}

func (s *server) ingest(ctx context.Context, api, facility string) error {
	logger.Info("ingestion is starting")
	defer logger.Info("ingestion is done")

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

		s.dbLock.Lock()
		s.dbReady = true
		s.dbLock.Unlock()
		return nil
	}

	return errors.New("maximum fetch/copy errors reached")
}
