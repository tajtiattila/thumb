package thumb

import (
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
)

var ErrNotFound = errors.New("thumb: key not found")

type KeyValue interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
	Close() error
}

func NewMemoryKeyValue() KeyValue {
	return make(memkv)
}

type memkv map[string][]byte

func (m memkv) Get(key string) ([]byte, error) {
	v, ok := m[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (m memkv) Put(key string, value []byte) error {
	v := make([]byte, len(value))
	copy(v, value)
	m[key] = v
	return nil
}

func (memkv) Close() error { return nil }

func OpenLevelDBKeyValue(path string) (KeyValue, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &levelkv{db}, nil
}

type levelkv struct {
	db *leveldb.DB
}

func (l *levelkv) Get(key string) ([]byte, error) {
	v, err := l.db.Get([]byte(key), nil)
	if err == leveldb.ErrNotFound {
		err = ErrNotFound
	}
	return v, err
}

func (l *levelkv) Put(key string, value []byte) error {
	return l.db.Put([]byte(key), value, nil)
}

func (l *levelkv) Close() error {
	return l.db.Close()
}
