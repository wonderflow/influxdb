package tsdb

import (
	"encoding/binary"
	"strings"

	"github.com/influxdb/influxdb/influxql"
	tsm "github.com/influxdb/influxdb/tsdb/engine/tsm1"
)

type ShardReader interface {
	Open() error
	Next() bool
	Read() (string, []tsm.Value, error)
	Close() error
}

// Cursor represents an iterator over a series.
type Cursor interface {
	SeekTo(seek int64) (key int64, value interface{})
	Next() (key int64, value interface{})
}

type Field struct {
	ID   uint8             `json:"id,omitempty"`
	Name string            `json:"name,omitempty"`
	Type influxql.DataType `json:"type,omitempty"`
}

type MeasurementFields struct {
	Fields map[string]*Field `json:"fields"`
	Codec  *FieldCodec
}

type Series struct {
	Key  string
	Tags map[string]string
}

func MeasurementFromSeriesKey(key string) string {
	idx := strings.Index(key, ",")
	if idx == -1 {
		return key
	}
	return key[:strings.Index(key, ",")]
}

// DecodeKeyValue decodes the key and value from bytes.
func DecodeKeyValue(field string, dec *FieldCodec, k, v []byte) (int64, interface{}) {
	// Convert key to a timestamp.
	key := int64(btou64(k[0:8]))

	decValue, err := dec.DecodeByName(field, v)
	if err != nil {
		return key, nil
	}
	return key, decValue
}

// btou64 converts an 8-byte slice into an uint64.
func btou64(b []byte) uint64 { return binary.BigEndian.Uint64(b) }
