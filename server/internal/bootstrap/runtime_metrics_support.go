package bootstrap

import (
	"database/sql"
	"path/filepath"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
)

// InitMetrics initializes metrics services.
func InitMetrics(dataDir string, sharedDB *sql.DB) (*metrics.Collector, *metrics.MetricsWriter) {
	return InitMetricsWithReadDB(dataDir, sharedDB, sharedDB)
}

// InitMetricsWithReadDB initializes metrics services with separate write/read
// handles when metrics persistence reuses the shared application database.
func InitMetricsWithReadDB(dataDir string, sharedDB, readDB *sql.DB) (*metrics.Collector, *metrics.MetricsWriter) {
	metricsCollector := metrics.NewCollector(5*time.Second, 120)
	metricsCollector.Start()

	metricsConfig := metrics.DefaultWriterConfig()
	if sharedDB != nil {
		metricsConfig.SharedSQLiteDB = sharedDB
		metricsConfig.SharedSQLiteReadDB = readDB
	} else {
		metricsConfig.SQLiteDBPath = filepath.Join(dataDir, "metrics.db")
	}
	metricsWriter := metrics.NewMetricsWriter(nil, metricsConfig)
	metricsWriter.Start()

	return metricsCollector, metricsWriter
}
