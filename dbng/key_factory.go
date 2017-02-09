package dbng

import (
	"crypto/rand"
	"encoding/base64"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db"
)

type KeyFactory interface {
	GetOrCreateKey() (string, error)
	SetKey(string) error
}

type keyFactory struct {
	conn Conn
	db   *db.SQLDB
}

func NewKeyFactory(conn Conn) KeyFactory {
	return &keyFactory{
		conn: conn,
	}
}

func (factory *keyFactory) insertKey(key string) error {
	tx, err := factory.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = psql.Insert("keys").
		Columns("name", "key").
		Values("csrf", key).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (factory *keyFactory) GetOrCreateKey() (string, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return "", err
	}

	defer tx.Rollback()

	key, err := generateKey()
	if err != nil {
		return "", err
	}

	err = factory.insertKey(key)
	if err != nil {
		err = psql.Select("key").
			From("keys").
			Where(sq.Eq{"name": "csrf"}).
			RunWith(tx).
			QueryRow().
			Scan(&key)
		if err != nil {
			return "", err
		}
	}

	err = tx.Commit()
	if err != nil {
		return "", err
	}

	return key, nil
}

func (factory *keyFactory) SetKey(key string) error {
	tx, err := factory.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = factory.insertKey(key)
	if err != nil {
		_, err = psql.Update("keys").
			Set("key", key).
			Where(sq.Eq{"name": "csrf"}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func generateKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
