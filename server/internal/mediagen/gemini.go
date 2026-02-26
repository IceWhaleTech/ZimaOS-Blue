package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

// GeminiProvider implements MediaProvider using Google's Gemini generateContent API.
// Gemini image generation is synchronous — Generate blocks until the image is ready.
type GeminiProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	tasks   sync.Map // taskID -> *MediaTask (for Poll lookups)
}

// NewGeminiProvider creates a new Gemini media provider.
func NewGeminiProvider(apiKey, baseURL string) *GeminiProvider {
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	return &GeminiProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		{ID: "gemini-2.0-flash-exp-image-generation", Name: "Gemini Flash Image", Type: MediaTypeImage, Provider: "gemini"},
		{ID: "imagen-3.0-generate-002", Name: "Imagen 3", Type: MediaTypeImage, Provider: "gemini"},
	}
}

func (p *GeminiProvider) SupportsType(t MediaType) bool {
	return t == MediaTypeImage
}

// Generate calls Gemini's generateContent with IMAGE response modality.
// This is synchronous — blocks until the image is generated.
func (p *GeminiProvider) Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	model := req.Model
	if model == "" {
		model = "gemini-2.0-flash-exp-image-generation"
	}

	// Build Gemini request
	geminiReq := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: req.Prompt},
				},
			},
		},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"IMAGE", "TEXT"},
		},
	}

	// Add reference image if provided
	if len(req.ReferenceImage) > 0 {
		geminiReq.Contents[0].Parts = append([]geminiPart{
			{
				InlineData: &geminiInlineData{
					MimeType: "image/png",
					Data:     string(req.ReferenceImage),
				},
			},
		}, geminiReq.Contents[0].Parts...)
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.baseURL, model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini response parse error: %w", err)
	}

	// Extract image data from response
	var results []MediaResult
	for _, candidate := range geminiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				results = append(results, MediaResult{
					B64JSON:     part.InlineData.Data,
					ContentType: part.InlineData.MimeType,
				})
			}
		}
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	taskID := uuid.New().String()
	now := timeutil.NowTime()
	task := &MediaTask{
		BaseTask: task.BaseTask{
			ID:          taskID,
			Status:      TaskStatusSucceeded,
			Progress:    1.0,
			CreatedAt:   now,
			CompletedAt: &now,
		},
		Type:     MediaTypeImage,
		Provider: "gemini",
		Model:    model,
		Response: &MediaResponse{
			Created: now.Unix(),
			Data:    results,
		},
	}

	p.tasks.Store(taskID, task)
	return task, nil
}

// Poll returns a stored completed task. Gemini is synchronous so this is a lookup only.
func (p *GeminiProvider) Poll(_ context.Context, taskID string) (*MediaTask, error) {
	v, ok := p.tasks.Load(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	return v.(*MediaTask), nil
}

// Gemini API types

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig geminiGenConfig  `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenConfig struct {
	ResponseModalities []string `json:"responseModalities,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}
