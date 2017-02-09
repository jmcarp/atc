package migrations

import "github.com/concourse/atc/dbng/migration"

func AddKeysTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	CREATE TABLE keys (
		name text PRIMARY KEY,
		key text
	)
	`)
	if err != nil {
		return err
	}

	return nil
}
