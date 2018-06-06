package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"
)

func truncate(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return errors.Wrap(err, "BEGIN transaction")
	}

	_, err = tx.Exec("TRUNCATE hardware")
	if err != nil {
		return errors.Wrap(err, "TRUNCATE")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "TRUNCATE")
	}
	return err
}

func copyin(db *sql.DB, data []map[string]interface{}) error {
	now := time.Now()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return errors.Wrap(err, "BEGIN transaction")
	}

	q := pq.CopyIn("hardware", "data")
	q += " CSV QUOTE e'\\x01' DELIMITER e'\\x02';"
	stmt, err := tx.Prepare(q)
	if err != nil {
		return errors.Wrap(err, "PREPARE COPY IN")
	}

	for _, j := range data {
		var q []byte
		q, err = json.Marshal(j)
		if err != nil {
			return errors.Wrap(err, "marshal json")
		}
		_, err = stmt.Exec(string(q))
		if err != nil {
			return errors.Wrap(err, "COPYing 1 object")
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return errors.Wrap(err, "empty EXEC to notify lib/pq")
	}

	err = stmt.Close()
	if err != nil {
		return errors.Wrap(err, "performing COPY")
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
