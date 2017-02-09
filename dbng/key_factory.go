package dbng

import (
	"crypto/rand"
	"encoding/base64"

	sq "github.com/Masterminds/squirrel"
)

type KeyFactory interface {
	GetOrCreateKey() (string, error)
}

type keyFactory struct {
	conn Conn
}

func NewKeyFactory(conn Conn) KeyFactory {
	return &keyFactory{
		conn: conn,
	}
}

func (factory *keyFactory) GetOrCreateKey() (string, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return "", err
	}

	defer tx.Rollback()

	var key string

	err = psql.Select("key").
		From("keys").
		Where(sq.Eq{"name": "csrf"}).
		RunWith(tx).
		QueryRow().
		Scan(&key)
	if err != nil {
		key, err = generateKey()
		if err != nil {
			return "", err
		}

		_, err = psql.Insert("keys").
			Columns("name", "key").
			Values("csrf", key).
			RunWith(tx).
			Exec()
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

func generateKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
