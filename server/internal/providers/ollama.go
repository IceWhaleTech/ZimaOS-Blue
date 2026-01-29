// Package providers implements auto-detection for various LLM providers.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaModel represents an Ollama model.
type OllamaModel struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	Details    struct {
		Format            string   `json:"format"`
		Family            string   `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
	} `json:"details"`
}

// OllamaInfo contains information about a detected Ollama instance.
type OllamaInfo struct {
	Available bool          `json:"available"`
	Endpoint  string        `json:"endpoint"`
	Version   string        `json:"version"`
	Models    []OllamaModel `json:"models"`
}

// OllamaDetector detects Ollama installations.
type OllamaDetector struct {
	Endpoints []string
	Timeout   time.Duration
	client    *http.Client
}

// NewOllamaDetector creates a new Ollama detector with default settings.
func NewOllamaDetector() *OllamaDetector {
	return &OllamaDetector{
		Endpoints: []string{
			"http://localhost:11434",
			"http://127.0.0.1:11434",
		},
		Timeout: 5 * time.Second,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Detect attempts to detect Ollama on configured endpoints.
func (d *OllamaDetector) Detect(ctx context.Context) (*OllamaInfo, error) {
	for _, endpoint := range d.Endpoints {
		info, err := d.detectAt(ctx, endpoint)
		if err == nil && info.Available {
			return info, nil
		}
	}

	return &OllamaInfo{
		Available: false,
	}, nil
}

// DetectAt attempts to detect Ollama at a specific endpoint.
func (d *OllamaDetector) DetectAt(ctx context.Context, endpoint string) (*OllamaInfo, error) {
	return d.detectAt(ctx, endpoint)
}

func (d *OllamaDetector) detectAt(ctx context.Context, endpoint string) (*OllamaInfo, error) {
	info := &OllamaInfo{
		Available: false,
		Endpoint:  endpoint,
	}

	// Check version endpoint
	versionURL := endpoint + "/api/version"
	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return info, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	// Parse version response
	var versionResp struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return info, err
	}
	info.Version = versionResp.Version

	// Get models
	models, err := d.getModels(ctx, endpoint)
	if err != nil {
		// Ollama is available but we couldn't get models
		info.Available = true
		return info, nil
	}

	info.Available = true
	info.Models = models
	return info, nil
}

func (d *OllamaDetector) getModels(ctx context.Context, endpoint string) ([]OllamaModel, error) {
	tagsURL := endpoint + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", tagsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tagsResp struct {
		Models []OllamaModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, err
	}

	return tagsResp.Models, nil
}

// HealthCheck performs a health check on Ollama.
func (d *OllamaDetector) HealthCheck(ctx context.Context, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/api/version", nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetModelNames returns just the model names from detected models.
func (info *OllamaInfo) GetModelNames() []string {
	names := make([]string, len(info.Models))
	for i, m := range info.Models {
		names[i] = m.Name
	}
	return names
}
