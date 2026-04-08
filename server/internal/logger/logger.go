package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/rs/zerolog"
)

var (
	log              zerolog.Logger
	managedClosersMu sync.Mutex
	managedClosers   []io.Closer
)

type initOptions struct {
	additionalWriters []io.Writer
}

type InitOption func(*initOptions)

func WithAdditionalWriter(writer io.Writer) InitOption {
	return func(options *initOptions) {
		if writer == nil {
			return
		}
		options.additionalWriters = append(options.additionalWriters, writer)
	}
}

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

func Init(cfg *config.LogConfig, opts ...InitOption) error {
	_ = Close()

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
	closers := make([]io.Closer, 0, 1)
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
		closers = appendUniqueCloser(closers, f)
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

	var options initOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	for _, writer := range options.additionalWriters {
		closers = appendManagedWriterCloser(closers, writer)
	}

	// Write durable/raw sinks first. If stdout/stderr becomes unavailable during
	// detached runs, later terminal-write failures will not prevent the buffer or
	// additional mirror writers from receiving the entry.
	writers := make([]io.Writer, 0, 2+len(options.additionalWriters))
	writers = append(writers, GetBuffer())
	writers = append(writers, options.additionalWriters...)
	writers = append(writers, output)
	multiWriter := io.MultiWriter(writers...)
	setManagedClosers(closers)

	log = zerolog.New(multiWriter).With().Timestamp().Caller().Logger()
	return nil
}

func InitWithMirror(cfg *config.LogConfig, mirrorPath string) error {
	mirrorPath = strings.TrimSpace(mirrorPath)
	if mirrorPath == "" || sameLogTarget(cfg.Output, mirrorPath) {
		return Init(cfg)
	}

	if err := os.MkdirAll(filepath.Dir(mirrorPath), 0750); err != nil {
		return err
	}
	f, err := os.OpenFile(mirrorPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	return Init(cfg, WithAdditionalWriter(f))
}

func Close() error {
	managedClosersMu.Lock()
	closers := managedClosers
	managedClosers = nil
	managedClosersMu.Unlock()

	var firstErr error
	for _, closer := range closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func setManagedClosers(closers []io.Closer) {
	managedClosersMu.Lock()
	defer managedClosersMu.Unlock()
	managedClosers = closers
}

func appendManagedWriterCloser(closers []io.Closer, writer io.Writer) []io.Closer {
	closer, ok := writer.(io.Closer)
	if !ok {
		return closers
	}
	if file, ok := writer.(*os.File); ok {
		if file == os.Stdout || file == os.Stderr {
			return closers
		}
	}
	return appendUniqueCloser(closers, closer)
}

func appendUniqueCloser(closers []io.Closer, closer io.Closer) []io.Closer {
	for _, existing := range closers {
		if existing == closer {
			return closers
		}
	}
	return append(closers, closer)
}

func sameLogTarget(configuredOutput, mirrorPath string) bool {
	switch strings.TrimSpace(configuredOutput) {
	case "", "stdout", "stderr":
		return false
	}

	configuredAbs, err := filepath.Abs(configuredOutput)
	if err != nil {
		return false
	}
	mirrorAbs, err := filepath.Abs(mirrorPath)
	if err != nil {
		return false
	}
	return filepath.Clean(configuredAbs) == filepath.Clean(mirrorAbs)
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
