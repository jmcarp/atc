package dbng

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

var ErrWorkerBaseResourceTypeAlreadyExists = errors.New("worker base resource type already exists")

// base_resource_types: <- gced referenced by 0 workers
// | id | type | image | version |

// worker_resource_types: <- synced w/ worker creation
// | worker_name | base_resource_type_id |

// resource_caches: <- gced by cache collector
// | id | resource_cache_id | base_resource_type_id | source_hash | params_hash | version |

type WorkerResourceType struct {
	Worker  *Worker
	Image   string // The path to the image, e.g. '/opt/concourse/resources/git'.
	Version string // The version of the image, e.g. a SHA of the rootfs.

	BaseResourceType *BaseResourceType
}

type UsedWorkerResourceType struct {
	Worker *Worker

	UsedBaseResourceType *UsedBaseResourceType
}

func (wrt WorkerResourceType) FindOrCreate(tx Tx) (*UsedWorkerResourceType, error) {
	usedBaseResourceType, err := wrt.BaseResourceType.FindOrCreate(tx)
	if err != nil {
		return nil, err
	}

	uwrt, found, err := wrt.find(tx, usedBaseResourceType)
	if err != nil {
		return nil, err
	}

	if found {
		return uwrt, nil
	}

	return wrt.create(tx, usedBaseResourceType)
}

func (wrt WorkerResourceType) find(tx Tx, usedBaseResourceType *UsedBaseResourceType) (*UsedWorkerResourceType, bool, error) {
	var worker_name string
	err := psql.Select("worker_name").From("worker_base_resource_types").Where(sq.Eq{
		"worker_name":           wrt.Worker.Name,
		"base_resource_type_id": usedBaseResourceType.ID,
		"image":                 wrt.Image,
		"version":               wrt.Version,
	}).RunWith(tx).QueryRow().Scan(&worker_name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &UsedWorkerResourceType{
		Worker:               wrt.Worker,
		UsedBaseResourceType: usedBaseResourceType,
	}, true, nil
}

func (wrt WorkerResourceType) create(tx Tx, usedBaseResourceType *UsedBaseResourceType) (*UsedWorkerResourceType, error) {
	_, err := psql.Insert("worker_base_resource_types").
		Columns(
			"worker_name",
			"base_resource_type_id",
			"image",
			"version",
		).
		Values(
			wrt.Worker.Name,
			usedBaseResourceType.ID,
			wrt.Image,
			wrt.Version,
		).
		RunWith(tx).
		Exec()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return nil, ErrWorkerBaseResourceTypeAlreadyExists
		}

		return nil, err
	}

	return &UsedWorkerResourceType{
		Worker:               wrt.Worker,
		UsedBaseResourceType: usedBaseResourceType,
	}, nil
}
