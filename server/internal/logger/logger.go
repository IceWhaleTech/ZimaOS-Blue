package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
)

var log zerolog.Logger

// DefaultBufferSize is the default number of log entries to keep in memory
const DefaultBufferSize = 5000

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
	colorGray    = "\033[90m"
)

func Init(cfg *config.LogConfig) error {
	// Initialize the log buffer
	InitBuffer(DefaultBufferSize)

	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set output
	var output io.Writer
	switch cfg.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		output = f
	}

	// For console format, wrap the output with ConsoleWriter
	// but keep the buffer receiving raw JSON
	if cfg.Format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			FormatLevel: func(i interface{}) string {
				level := strings.ToUpper(fmt.Sprintf("%s", i))
				switch level {
				case "DEBUG":
					return colorGray + "DBG" + colorReset
				case "INFO":
					return colorGreen + "INF" + colorReset
				case "WARN":
					return colorYellow + "WRN" + colorReset
				case "ERROR":
					return colorRed + "ERR" + colorReset
				case "FATAL":
					return colorRed + "FTL" + colorReset
				default:
					return level
				}
			},
			FormatMessage: func(i interface{}) string {
				return colorCyan + fmt.Sprintf("%s", i) + colorReset
			},
			FormatFieldName: func(i interface{}) string {
				return colorBlue + fmt.Sprintf("%s", i) + colorReset + "="
			},
			FormatFieldValue: func(i interface{}) string {
				field := fmt.Sprintf("%s", i)
				// Color tag field values
				if strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]") {
					return colorMagenta + field + colorReset
				}
				return field
			},
		}
	}

	// Create a multi-writer to write to both the output and the ring buffer
	// Note: zerolog writes JSON to all writers, ConsoleWriter converts it for display
	multiWriter := io.MultiWriter(output, GetBuffer())

	log = zerolog.New(multiWriter).With().Timestamp().Caller().Logger()
	return nil
}

func Get() *zerolog.Logger {
	return &log
}

func Debug() *zerolog.Event {
	return log.Debug()
}

func Info() *zerolog.Event {
	return log.Info()
}

func Warn() *zerolog.Event {
	return log.Warn()
}

func Error() *zerolog.Event {
	return log.Error()
}

func Fatal() *zerolog.Event {
	return log.Fatal()
}

func With() zerolog.Context {
	return log.With()
}
