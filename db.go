package main

import (
	"context"
	"database/sql"

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
