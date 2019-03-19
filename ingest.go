package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

func fetchFacilityPage(ctx context.Context, client *packngo.Client, url string) ([]map[string]interface{}, uint, uint, error) {
	req, err := client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, 0, errors.Wrap(err, "failed to create fetch request")
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
		return nil, 0, 0, errors.Wrap(err, "failed to fetch page")
	}

	return r.Hardware, uint(r.Meta.LastPage), uint(r.Meta.Total), nil
}

func fetchFacility(ctx context.Context, client *packngo.Client, api, facility string) ([]map[string]interface{}, error) {
	logger.Info("fetch start")
	labels := prometheus.Labels{"method": "Ingest", "op": "fetch"}
	ingestCount.With(labels).Inc()
	timer := prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(labels).Set))

	var j []map[string]interface{}
	base := api + "staff/cacher/hardware?facility=" + facility + "&sort_by=created_at&sort_direction=asc&per_page=50&page="
	for page, lastPage := uint(1), uint(1); page <= lastPage; page++ {
		url := base + strconv.Itoa(int(page))
		hw, last, total, err := fetchFacilityPage(ctx, client, url)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch page %d of %d", page, lastPage)
		}
		lastPage = last

		if j == nil {
			j = make([]map[string]interface{}, 0, total)
		}

		j = append(j, hw...)
		logger.With("have", len(j), "want", total).Info("fetched a page")
	}

	timer.ObserveDuration()
	logger.Info("fetch done")

	return j, nil
}

func copyin(ctx context.Context, db *sql.DB, data []map[string]interface{}) error {
	logger.Info("copy start")
	labels := prometheus.Labels{"method": "Ingest", "op": "copy"}
	ingestCount.With(labels).Inc()
	timer := prometheus.NewTimer(prometheus.ObserverFunc(ingestDuration.With(labels).Set))

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

	timer.ObserveDuration()
	logger.Info("copy done")

	return nil
}

func (s *server) ingest(ctx context.Context, api, facility string) error {
	logger.Info("ingestion is starting")
	defer logger.Info("ingestion is done")

	labels := prometheus.Labels{"method": "Ingest", "op": ""}
	cacheInFlight.With(labels).Inc()
	defer cacheInFlight.With(labels).Dec()

	var errCount int
	for errCount = 0; errCount < getMaxErrs(); errCount++ {
		data, err := fetchFacility(ctx, s.packet, api, facility)
		if err != nil {
			labels = prometheus.Labels{"method": "Ingest", "op": "fetch"}
			ingestErrors.With(labels).Inc()
			logger.With("error", err).Info()

			if ctx.Err() == context.Canceled {
				return nil
			}

			time.Sleep(5 * time.Second)
			continue
		}

		if err = copyin(ctx, s.db, data); err != nil {
			labels = prometheus.Labels{"method": "Ingest", "op": "copy"}
			ingestErrors.With(labels).Inc()

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

		s.dbLock.Lock()
		s.dbReady = true
		s.dbLock.Unlock()
		return nil
	}

	return errors.New("maximum fetch/copy errors reached")
}
