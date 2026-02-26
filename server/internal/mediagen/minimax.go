package mediagen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const defaultMiniMaxBaseURL = "https://api.minimax.chat"

// MiniMaxProvider implements MediaProvider using MiniMax's Hailuo video generation API.
// Uses async task pattern: POST to create → GET to poll status.
type MiniMaxProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewMiniMaxProvider creates a new MiniMax media provider.
func NewMiniMaxProvider(apiKey, baseURL string) *MiniMaxProvider {
	if baseURL == "" {
		baseURL = defaultMiniMaxBaseURL
	}
	return &MiniMaxProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *MiniMaxProvider) Name() string { return "minimax-media" }

func (p *MiniMaxProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		// --- Text-to-Video (t2v) ---
		{ID: "MiniMax-Hailuo-2.3", Name: "Hailuo 2.3", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "minimax-media"},
		{ID: "MiniMax-Hailuo-2.3-Fast", Name: "Hailuo 2.3 Fast", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "minimax-media"},
		{ID: "MiniMax-Hailuo-02", Name: "Hailuo 02", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "minimax-media"},
		{ID: "T2V-01", Name: "Hailuo T2V-01", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "minimax-media"},
		{ID: "T2V-01-Director", Name: "Hailuo T2V-01 Director", Type: MediaTypeVideo, Category: CategoryT2V, Provider: "minimax-media"},
		// --- Image-to-Video (i2v) ---
		{ID: "I2V-01", Name: "Hailuo I2V-01", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "minimax-media"},
		{ID: "I2V-01-Director", Name: "Hailuo I2V-01 Director", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "minimax-media"},
		{ID: "I2V-01-live", Name: "Hailuo I2V-01 Live", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "minimax-media"},
		{ID: "S2V-01", Name: "Hailuo S2V-01", Type: MediaTypeVideo, Category: CategoryI2V, Provider: "minimax-media"},
	}
}

func (p *MiniMaxProvider) SupportsType(t MediaType) bool {
	return t == MediaTypeVideo
}

// Generate creates an async video generation task on MiniMax.
func (p *MiniMaxProvider) Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	model := req.Model
	if model == "" {
		model = "MiniMax-Hailuo-2.3"
	}

	mmReq := map[string]any{
		"model":  model,
		"prompt": req.Prompt,
	}

	// Image-to-video: pass first_frame_image
	if req.ReferenceURL != "" {
		mmReq["first_frame_image"] = req.ReferenceURL
	} else if len(req.ReferenceURLs) > 0 {
		mmReq["first_frame_image"] = req.ReferenceURLs[0]
	}

	body, err := json.Marshal(mmReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/video_generation", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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
		return nil, fmt.Errorf("minimax API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var mmResp miniMaxCreateResponse
	if err := json.Unmarshal(respBody, &mmResp); err != nil {
		return nil, fmt.Errorf("minimax response parse error: %w", err)
	}

	if mmResp.TaskID == "" {
		return nil, fmt.Errorf("minimax: no task_id in response: %s", truncate(string(respBody), 200))
	}

	return &MediaTask{
		BaseTask:   task.BaseTask{ID: uuid.New().String(), Status: TaskStatusProcessing, CreatedAt: timeutil.NowTime()},
		Type:       MediaTypeVideo,
		UpstreamID: mmResp.TaskID,
	}, nil
}

// Poll checks the status of an async MiniMax video generation task.
func (p *MiniMaxProvider) Poll(ctx context.Context, taskID string) (*MediaTask, error) {
	url := fmt.Sprintf("%s/api/v1/query/video_generation?task_id=%s", p.baseURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var mmResp miniMaxPollResponse
	if err := json.Unmarshal(respBody, &mmResp); err != nil {
		return nil, fmt.Errorf("minimax poll parse error: %w (body: %s)", err, truncate(string(respBody), 200))
	}

	t := &MediaTask{UpstreamID: taskID}

	switch mmResp.Status {
	case "Success":
		t.Status = TaskStatusSucceeded
		var results []MediaResult
		if mmResp.FileID != "" {
			// MiniMax returns a file_id; the download URL is at /api/v1/files/retrieve?file_id=xxx
			downloadURL := fmt.Sprintf("%s/api/v1/files/retrieve?file_id=%s", p.baseURL, mmResp.FileID)
			results = append(results, MediaResult{
				OriginalURL: downloadURL,
				ContentType: "video/mp4",
			})
		}
		now := timeutil.NowTime()
		t.Response = &MediaResponse{Created: now.Unix(), Data: results}
		t.CompletedAt = &now
	case "Failed":
		t.Status = TaskStatusFailed
		t.Error = mmResp.BaseResp.StatusMsg
	case "Preparing", "Processing", "Queueing":
		t.Status = TaskStatusProcessing
	default:
		t.Status = TaskStatusProcessing
	}

	return t, nil
}

// MiniMax API types

type miniMaxCreateResponse struct {
	TaskID   string `json:"task_id"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

type miniMaxPollResponse struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	FileID   string `json:"file_id,omitempty"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}
