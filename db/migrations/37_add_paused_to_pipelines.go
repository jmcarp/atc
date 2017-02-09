package migrations

import "github.com/concourse/atc/dbng/migration"

func AddPausedToPipelines(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN paused boolean DEFAULT(false);
`)

	return err

}
