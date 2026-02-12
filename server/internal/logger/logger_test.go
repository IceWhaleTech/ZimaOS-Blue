package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestInit_DefaultLevel(t *testing.T) {
	cfg := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("GlobalLevel() = %v, want %v", zerolog.GlobalLevel(), zerolog.InfoLevel)
	}
}

func TestInit_DebugLevel(t *testing.T) {
	cfg := &config.LogConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Errorf("GlobalLevel() = %v, want %v", zerolog.GlobalLevel(), zerolog.DebugLevel)
	}
}

func TestInit_InvalidLevel(t *testing.T) {
	cfg := &config.LogConfig{
		Level:  "invalid",
		Format: "json",
		Output: "stdout",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Should default to info level
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("GlobalLevel() = %v, want %v", zerolog.GlobalLevel(), zerolog.InfoLevel)
	}
}

func TestLogOutput(t *testing.T) {
	var buf bytes.Buffer

	log = zerolog.New(&buf).With().Timestamp().Logger()

	Info().Str("key", "value").Msg("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Log output does not contain message: %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("Log output does not contain key-value: %s", output)
	}
}

func TestGet(t *testing.T) {
	cfg := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	Init(cfg)

	logger := Get()
	if logger == nil {
		t.Error("Get() returned nil")
	}
}
