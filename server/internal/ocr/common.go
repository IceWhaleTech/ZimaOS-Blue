package ocr

import (
	"context"
	"net/http"
	"strings"
)

type Result struct {
	Text           string   `json:"text"`
	Engine         string   `json:"engine,omitempty"`
	Model          string   `json:"model,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
	AutoDownloaded []string `json:"auto_downloaded,omitempty"`
}

type Config struct {
	ModelDir        string
	AutoDownload    bool
	WorkerCount     uint
	PreferredModels []string
	HTTPClient      *http.Client
}

type Service interface {
	Extract(ctx context.Context, imagePNG []byte) (Result, error)
	Close() error
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
}
