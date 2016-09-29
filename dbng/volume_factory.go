package dbng

import (
	"errors"

	sq "github.com/Masterminds/squirrel"
)

type VolumeFactory struct {
	conn Conn
}

func NewVolumeFactory(conn Conn) *VolumeFactory {
	return &VolumeFactory{
		conn: conn,
	}
}

func (factory *VolumeFactory) CreateResourceCacheVolume(team *Team, worker *Worker, resourceCache *UsedResourceCache) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()
	return factory.createVolume(tx, team.ID, worker, map[string]interface{}{"resource_cache_id": resourceCache.ID})
}

func (factory *VolumeFactory) CreateBaseResourceTypeVolume(team *Team, worker *Worker, ubrt *UsedBaseResourceType) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	return factory.createVolume(tx, team.ID, worker, map[string]interface{}{
		"base_resource_type_id": ubrt.ID,
		"initialized":           true,
	})
}

func (factory *VolumeFactory) CreateContainerVolume(team *Team, worker *Worker, container *CreatingContainer, mountPath string) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	return factory.createVolume(tx, team.ID, worker, map[string]interface{}{
		"container_id": container.ID,
		"path":         mountPath,
		"initialized":  true,
	})
}

func (factory *VolumeFactory) GetOrphanedVolumes() ([]*CreatedVolume, []*DestroyingVolume, error) {
	query, args, err := psql.Select("v.id, v.handle, v.state, w.name, w.addr").
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		Where(sq.Eq{
			"v.resource_cache_id":     nil,
			"v.base_resource_type_id": nil,
			"v.container_id":          nil,
		}).
		ToSql()
	if err != nil {
		return nil, nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	createdVolumes := []*CreatedVolume{}
	destroyingVolumes := []*DestroyingVolume{}

	for rows.Next() {
		var id int
		var handle string
		var state string
		var workerName string
		var workerAddress string

		err = rows.Scan(&id, &handle, &state, &workerName, &workerAddress)
		if err != nil {
			return nil, nil, err
		}
		switch state {
		case VolumeStateCreated:
			createdVolumes = append(createdVolumes, &CreatedVolume{
				ID:     id,
				Handle: handle,
				Worker: &Worker{
					Name:       workerName,
					GardenAddr: workerAddress,
				},
				conn: factory.conn,
			})
		case VolumeStateDestroying:
			destroyingVolumes = append(destroyingVolumes, &DestroyingVolume{
				ID:     id,
				Handle: handle,
				Worker: &Worker{
					Name:       workerName,
					GardenAddr: workerAddress,
				},
				conn: factory.conn,
			})
		}
	}

	return createdVolumes, destroyingVolumes, nil
}

// 1. open tx
// 2. lookup cache id
//   * if not found, create.
//     * if fails (unique violation; concurrent create), goto 1.
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; preexisting cache id was removed), goto 1.
// 4. commit tx

var ErrWorkerResourceTypeNotFound = errors.New("worker resource type no longer exists (stale?)")

// 1. open tx
// 2. lookup worker resource type id
//   * if not found, fail; worker must have new version or no longer supports type
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; worker type gone), fail for same reason as 2.
// 4. commit tx
func (factory *VolumeFactory) createVolume(tx Tx, teamID int, worker *Worker, columns map[string]interface{}) (*CreatingVolume, error) {
	var volumeID int
	columnNames := []string{"team_id", "worker_name"}
	columnValues := []interface{}{teamID, worker.Name}
	for name, value := range columns {
		columnNames = append(columnNames, name)
		columnValues = append(columnValues, value)
	}

	err := psql.Insert("volumes").
		Columns(columnNames...).
		Values(columnValues...).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&volumeID)
	if err != nil {
		// TODO: explicitly handle fkey constraint on wrt id
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &CreatingVolume{
		Worker: worker,

		ID: volumeID,

		conn: factory.conn,
	}, nil
}
