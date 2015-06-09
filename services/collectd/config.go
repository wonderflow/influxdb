package collectd

import (
	"time"
)

const (
	DefaultBindAddress = ":25826"

	DefaultDatabase = "collectd"

	DefaultRetentionPolicy = ""

	DefaultBatchSize = 5000

	DefaultBatchDuration = 10 * time.Second

	DefaultTypesDB = "/usr/share/collectd/types.db"
)

// Config represents a configuration for the collectd service.
type Config struct {
	Enabled         bool          `toml:"enabled"`
	BindAddress     string        `toml:"bind-address"`
	Database        string        `toml:"database"`
	RetentionPolicy string        `toml:"retention-policy"`
	BatchSize       int           `toml:"batch-size"`
	BatchDuration   time.Duration `toml:"batch-timeout"`
	TypesDB         string        `toml:"typesdb"`
}

// NewConfig returns a new instance of Config with defaults.
func NewConfig() Config {
	return Config{
		Enabled:         false,
		BindAddress:     DefaultBindAddress,
		Database:        DefaultDatabase,
		RetentionPolicy: DefaultRetentionPolicy,
		BatchSize:       DefaultBatchSize,
		BatchDuration:   DefaultBatchDuration,
		TypesDB:         DefaultTypesDB,
	}
}
