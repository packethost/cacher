package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"
)

func pqError(err error) *pq.Error {
	if pqErr, ok := errors.Cause(err).(*pq.Error); ok {
		return pqErr
	}
	return nil
}

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

func deleteFromDB(ctx context.Context, db *sql.DB, id string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return errors.Wrap(err, "BEGIN transaction")
	}

	_, err = tx.Exec(`
	UPDATE hardware
	SET
		deleted_at = NOW()
	WHERE
		id = $1;
	`, id)

	if err != nil {
		return errors.Wrap(err, "DELETE")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "COMMIT")
	}
	return nil
}

func insertIntoDB(ctx context.Context, db *sql.DB, data string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return errors.Wrap(err, "BEGIN transaction")
	}

	_, err = tx.Exec(`
	INSERT INTO
		hardware (inserted_at, id, data)
	VALUES
		($1, ($2::jsonb ->> 'id')::uuid, $2)
	ON CONFLICT (id)
	DO
	UPDATE SET
		(inserted_at, deleted_at, data) = ($1, NULL, $2);
	`, time.Now(), data)
	if err != nil {
		return errors.Wrap(err, "INSERT")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "COMMIT")
	}
	return nil
}

func get(ctx context.Context, db *sql.DB, query string, args ...interface{}) (string, error) {
	row := db.QueryRowContext(ctx, query, args...)

	buf := []byte{}
	err := row.Scan(&buf)
	if err == nil {
		return string(buf), nil
	}

	if err != sql.ErrNoRows {
		err = errors.Wrap(err, "SELECT")
		sugar.Error(err)
	} else {
		err = nil
	}

	return "", err
}

func getByMAC(ctx context.Context, db *sql.DB, mac string) (string, error) {
	arg := `
	{
	  "network_ports": [
	    {
	      "data": {
		"mac": "` + mac + `"
	      }
	    }
	  ]
	}
	`
	query := `
	SELECT data
	FROM hardware
	WHERE
		deleted_at IS NULL
	AND
		data @> $1
	`

	return get(ctx, db, query, arg)
}

func getByIP(ctx context.Context, db *sql.DB, ip string) (string, error) {
	instance := `
	{
	  "instance": {
	    "ip_addresses": [
	      {
		"address": "` + ip + `"
	      }
	    ]
	  }
	}
	`
	hardwareOrManagement := `
	{
		"ip_addresses": [
			{
				"address": "` + ip + `"
			}
		]
	}
	`

	query := `
	SELECT data
	FROM hardware
	WHERE
		deleted_at IS NULL
	AND (
		data @> $1
		OR
		data @> $2
	)
	`

	return get(ctx, db, query, instance, hardwareOrManagement)
}

func getByID(ctx context.Context, db *sql.DB, id string) (string, error) {
	arg := id

	query := `
	SELECT data
	FROM hardware
	WHERE
		deleted_at IS NULL
	AND
		id = $1
	`
	return get(ctx, db, query, arg)
}

func getAll(db *sql.DB, fn func(string) error) error {
	rows, err := db.Query(`
	SELECT data
	FROM hardware
	WHERE
		deleted_at IS NULL
	`)

	if err != nil {
		return err
	}

	defer rows.Close()
	buf := []byte{}
	for rows.Next() {
		err = rows.Scan(&buf)
		if err != nil {
			err = errors.Wrap(err, "SELECT")
			sugar.Error(err)
			return err
		}

		sugar.Info("got data:", string(buf))
		err = fn(string(buf))
		if err != nil {
			return err
		}

	}

	err = rows.Err()
	if err == sql.ErrNoRows {
		err = nil
	}
	return err
}
