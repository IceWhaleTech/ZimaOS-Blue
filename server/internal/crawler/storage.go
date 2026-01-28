package crawler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Storage defines the interface for storing crawl results
type Storage interface {
	Save(results []Result) error
	Close() error
}

// JSONStorage saves results to a JSON file
type JSONStorage struct {
	filePath string
}

// NewJSONStorage creates a new JSON storage
func NewJSONStorage(filePath string) *JSONStorage {
	return &JSONStorage{filePath: filePath}
}

// Save writes results to a JSON file
func (s *JSONStorage) Save(results []Result) error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// Close implements Storage interface
func (s *JSONStorage) Close() error {
	return nil
}

// CSVStorage saves results to a CSV file
type CSVStorage struct {
	filePath string
}

// NewCSVStorage creates a new CSV storage
func NewCSVStorage(filePath string) *CSVStorage {
	return &CSVStorage{filePath: filePath}
}

// Save writes results to a CSV file
func (s *CSVStorage) Save(results []Result) error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write UTF-8 BOM for Excel compatibility
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"URL", "Title", "Status", "Links Count", "Text Preview", "Error", "Crawled At"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write data
	for _, r := range results {
		textPreview := r.Text
		if len(textPreview) > 200 {
			textPreview = textPreview[:200] + "..."
		}

		row := []string{
			r.URL,
			r.Title,
			fmt.Sprintf("%d", r.Status),
			fmt.Sprintf("%d", len(r.Links)),
			textPreview,
			r.Error,
			r.CrawledAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// Close implements Storage interface
func (s *CSVStorage) Close() error {
	return nil
}

// StreamingJSONStorage saves results incrementally to a JSON Lines file
type StreamingJSONStorage struct {
	file    *os.File
	encoder *json.Encoder
}

// NewStreamingJSONStorage creates a new streaming JSON storage
func NewStreamingJSONStorage(filePath string) (*StreamingJSONStorage, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &StreamingJSONStorage{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// SaveOne writes a single result to the file
func (s *StreamingJSONStorage) SaveOne(result Result) error {
	return s.encoder.Encode(result)
}

// Save writes multiple results to the file
func (s *StreamingJSONStorage) Save(results []Result) error {
	for _, r := range results {
		if err := s.SaveOne(r); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the file
func (s *StreamingJSONStorage) Close() error {
	return s.file.Close()
}

// GenerateFilename generates a filename with timestamp
func GenerateFilename(prefix, extension string) string {
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s.%s", prefix, timestamp, strings.TrimPrefix(extension, "."))
}
