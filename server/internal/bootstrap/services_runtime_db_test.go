package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOpenRuntimeDatabase_CreatesNewDatabase(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &ServerConfig{
		DataDir: tempDir,
	}
	logger := zap.NewNop()

	conn, err := openRuntimeDatabase(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	dbPath := filepath.Join(tempDir, "runtime.db")
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "runtime.db should be created")

	require.NotNil(t, conn.Writer)
	require.NotNil(t, conn.Reader)

	// Verify we can execute a simple query
	var result int
	err = conn.Writer.QueryRow("SELECT 1").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestOpenRuntimeDatabase_ReusesExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &ServerConfig{
		DataDir: tempDir,
	}
	logger := zap.NewNop()

	// First open - create the database
	conn1, err := openRuntimeDatabase(cfg, logger)
	require.NoError(t, err)

	// Create a test table
	_, err = conn1.Writer.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = conn1.Writer.Exec("INSERT INTO test_table (name) VALUES ('test')")
	require.NoError(t, err)
	conn1.Close()

	// Second open - should reuse existing database
	conn2, err := openRuntimeDatabase(cfg, logger)
	require.NoError(t, err)
	defer conn2.Close()

	// Verify data persists
	var name string
	err = conn2.Reader.QueryRow("SELECT name FROM test_table WHERE id = 1").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "test", name)
}

func TestInitServicesRuntimeDatabase(t *testing.T) {
	tempDir := t.TempDir()
	s := &Services{
		DataDir: tempDir,
		Logger:  zap.NewNop(),
	}
	cfg := &ServerConfig{
		DataDir: tempDir,
	}
	trace := NewStartupTrace("test", s.Logger)

	err := initServicesRuntimeDatabase(s, cfg, trace)
	require.NoError(t, err)
	require.NotNil(t, s.RuntimeDBConn)

	// Verify runtime.db exists
	dbPath := filepath.Join(tempDir, "runtime.db")
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}
