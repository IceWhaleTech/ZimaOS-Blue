package mediagen

import (
	"context"
	"testing"

	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
)

type captureMediaProvider struct {
	name  string
	type_ MediaType
	model string
	last  *MediaRequest
}

func (p *captureMediaProvider) Name() string { return p.name }

func (p *captureMediaProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{{ID: p.model, Name: p.model, Type: p.type_, Provider: p.name}}
}

func (p *captureMediaProvider) SupportsType(t MediaType) bool { return t == p.type_ }

func (p *captureMediaProvider) Generate(_ context.Context, req *MediaRequest) (*MediaTask, error) {
	cp := *req
	if req.Extra != nil {
		cp.Extra = make(map[string]any, len(req.Extra))
		for k, v := range req.Extra {
			cp.Extra[k] = v
		}
	}
	if len(req.ReferenceURLs) > 0 {
		cp.ReferenceURLs = append([]string(nil), req.ReferenceURLs...)
	}
	p.last = &cp
	resp := &MediaResponse{}
	if p.type_ == MediaTypeImage {
		resp.Data = []MediaResult{{B64JSON: fakeMediaImagePNGBase64, ContentType: "image/png"}}
		if cp.N > 1 {
			resp.Data = make([]MediaResult, cp.N)
			for i := 0; i < cp.N; i++ {
				resp.Data[i] = MediaResult{B64JSON: fakeMediaImagePNGBase64, ContentType: "image/png"}
			}
		}
	}
	return &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusSucceeded}, Response: resp}, nil
}

func (p *captureMediaProvider) Poll(_ context.Context, taskID string) (*MediaTask, error) {
	return nil, ErrTaskNotFound
}

func TestImageGenerateSkillSupportsNestedCamelCaseArgs(t *testing.T) {
	storage := NewMediaStorage(t.TempDir(), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	m := NewManager(storage, nil, "")
	p := &captureMediaProvider{name: "capture-image", type_: MediaTypeImage, model: "capture-image-model"}
	m.RegisterProvider(p)
	skill := NewImageGenerateSkill(m)

	out, err := skill.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"prompt":         "nested cat",
			"negativePrompt": "lowres",
			"model":          "capture-image-model",
			"numImages":      2,
			"aspectRatio":    "16:9",
			"referenceImage": "https://example.com/ref.png",
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	if got := data["status"]; got != "success" {
		t.Fatalf("status = %v, want success", got)
	}
	if p.last == nil {
		t.Fatal("expected provider request capture")
	}
	if p.last.Prompt != "nested cat" || p.last.NegativePrompt != "lowres" {
		t.Fatalf("request = %#v", p.last)
	}
	if p.last.N != 2 {
		t.Fatalf("N = %d, want 2", p.last.N)
	}
	if got := p.last.Extra["aspect_ratio"]; got != "16:9" {
		t.Fatalf("aspect_ratio = %v, want 16:9", got)
	}
	if p.last.ReferenceURL != "https://example.com/ref.png" {
		t.Fatalf("reference_url = %q, want https://example.com/ref.png", p.last.ReferenceURL)
	}
}

func TestVideoGenerateSkillSupportsNestedCamelCaseArgs(t *testing.T) {
	m := NewManager(nil, nil, "")
	p := &captureMediaProvider{name: "capture-video", type_: MediaTypeVideo, model: "capture-video-model"}
	m.RegisterProvider(p)
	skill := NewVideoGenerateSkill(m)

	out, err := skill.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"prompt":   "nested video",
			"model":    "capture-video-model",
			"duration": 9,
			"size":     "1280x720",
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	if got := data["status"]; got != string(TaskStatusProcessing) {
		t.Fatalf("status = %v, want %q", got, TaskStatusProcessing)
	}
	if p.last == nil {
		t.Fatal("expected provider request capture")
	}
	if p.last.Prompt != "nested video" || p.last.Duration != 9 || p.last.Size != "1280x720" {
		t.Fatalf("request = %#v", p.last)
	}
}
