package b1

import (
	"time"

	"github.com/boltdb/bolt"
	"github.com/influxdb/influxdb/tsdb/engine/tsm1"
)

type Reader struct {
	path string
	db   *bolt.DB
}

func NewReader(path string) *Reader {
	return &Reader{
		path: path,
	}
}

func (r *Reader) Open() error {
	// Open underlying storage.
	db, err := bolt.Open(r.path, 0666, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	r.db = db

	return nil
}

func (r *Reader) Next() bool {
	return false
}

func (r *Reader) Read() (string, []tsm1.Value, error) {
	return "", nil, nil
}

func (r *Reader) Close() error {
	return nil
}
