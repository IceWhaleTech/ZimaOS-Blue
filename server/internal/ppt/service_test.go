package ppt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
)

type fakeGenerator struct {
	hasProviders bool
	models       []mediagen.MediaModelInfo
	createReqs   []*mediagen.MediaRequest
	tasks        []*mediagen.MediaTask
	createErr    error
	waitErrs     []error
}

func (f *fakeGenerator) HasImageProviders() bool { return f.hasProviders }
func (f *fakeGenerator) Models() []mediagen.MediaModelInfo {
	return append([]mediagen.MediaModelInfo(nil), f.models...)
}
func (f *fakeGenerator) CreateTask(_ context.Context, req *mediagen.MediaRequest, _ string, category, source string) (*mediagen.MediaTask, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	clone := *req
	if req.Extra != nil {
		clone.Extra = map[string]any{}
		for k, v := range req.Extra {
			clone.Extra[k] = v
		}
	}
	f.createReqs = append(f.createReqs, &clone)
	id := len(f.createReqs)
	task := &mediagen.MediaTask{
		BaseTask: basetask.BaseTask{ID: "task-" + string(rune('0'+id)), Status: mediagen.TaskStatusProcessing},
		Category: category,
		Source:   source,
	}
	if len(f.tasks) >= id {
		return task, nil
	}
	return task, nil
}
func (f *fakeGenerator) WaitForTask(_ context.Context, taskID string) (*mediagen.MediaTask, error) {
	idx := len(f.createReqs) - 1
	if idx < 0 || idx >= len(f.tasks) {
		return nil, errors.New("missing fake task")
	}
	var err error
	if idx < len(f.waitErrs) {
		err = f.waitErrs[idx]
	}
	return f.tasks[idx], err
}

type fakeStorage struct {
	files map[string][]byte
}

func (f fakeStorage) ReadServedURL(servedURL string) ([]byte, error) {
	data, ok := f.files[servedURL]
	if !ok {
		return nil, errors.New("missing stored file")
	}
	return append([]byte(nil), data...), nil
}

type fakeReviewer struct {
	reviews []*ReviewResult
	err     error
	inputs  []ReviewOptions
}

func (f *fakeReviewer) ReviewImage(_ context.Context, _ string, opts ReviewOptions) (*ReviewResult, error) {
	f.inputs = append(f.inputs, opts)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.reviews) == 0 {
		return &ReviewResult{Overall: 90, Threshold: opts.Threshold, Pass: true}, nil
	}
	review := f.reviews[0]
	f.reviews = f.reviews[1:]
	return review, nil
}

func TestServicePrefersNanoBananaForTextOnly(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models: []mediagen.MediaModelInfo{
			{ID: "qwen-image-max", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I},
			{ID: "nano-banana-pro", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I},
		},
		tasks: []*mediagen.MediaTask{{
			BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
			Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
		}},
	}
	reviewer := &fakeReviewer{reviews: []*ReviewResult{{Overall: 91, Threshold: 80, Pass: true}}}
	svc := NewService(gen, fakeStorage{files: map[string][]byte{"/api/media/generated/images/one.png": []byte("png")}}, reviewer)

	result, err := svc.Generate(context.Background(), Request{Description: "Quarterly growth overview"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result.Model != "nano-banana-pro" {
		t.Fatalf("model = %q, want nano-banana-pro", result.Model)
	}
	if result.Mode != string(ModeTextOnly) {
		t.Fatalf("mode = %q, want %q", result.Mode, ModeTextOnly)
	}
	if len(gen.createReqs) != 1 {
		t.Fatalf("create calls = %d, want 1", len(gen.createReqs))
	}
	if got := gen.createReqs[0].Extra["aspect_ratio"]; got != DefaultAspectRatio {
		t.Fatalf("aspect_ratio = %v, want %q", got, DefaultAspectRatio)
	}
	if !strings.Contains(gen.createReqs[0].Prompt, "presentation-ready PPT slide visual") {
		t.Fatalf("prompt = %q", gen.createReqs[0].Prompt)
	}
}

func TestServiceFallsBackToTextModeWhenNoReferenceModel(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models: []mediagen.MediaModelInfo{
			{ID: "qwen-image-max", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I},
		},
		tasks: []*mediagen.MediaTask{{
			BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
			Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
		}},
	}
	reviewer := &fakeReviewer{reviews: []*ReviewResult{{Overall: 88, Threshold: 80, Pass: true}}}
	svc := NewService(gen, fakeStorage{files: map[string][]byte{"/api/media/generated/images/one.png": []byte("png")}}, reviewer)

	result, err := svc.Generate(context.Background(), Request{
		Description:     "Show a bold market landscape slide",
		ReferenceImages: []string{"https://example.com/ref.png"},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result.Mode != string(ModeTextOnly) {
		t.Fatalf("mode = %q, want text-only fallback", result.Mode)
	}
	if len(gen.createReqs[0].ReferenceURLs) != 0 {
		t.Fatalf("reference urls = %#v, want empty", gen.createReqs[0].ReferenceURLs)
	}
}

func TestServiceRetriesOnceWhenReviewIsBelowThreshold(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models:       []mediagen.MediaModelInfo{{ID: "nano-banana-pro", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I}},
		tasks: []*mediagen.MediaTask{
			{
				BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
				Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
			},
			{
				BaseTask: basetask.BaseTask{ID: "task-2", Status: mediagen.TaskStatusSucceeded},
				Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/two.png"}}},
			},
		},
	}
	reviewer := &fakeReviewer{reviews: []*ReviewResult{
		{Overall: 67, Threshold: 80, Pass: false, Issues: []ReviewIssue{{Severity: "major", Description: "Layout feels crowded"}}, Suggestions: []string{"Add more negative space"}},
		{Overall: 92, Threshold: 80, Pass: true, Suggestions: []string{"Looks polished"}},
	}}
	storage := fakeStorage{files: map[string][]byte{
		"/api/media/generated/images/one.png": []byte("one"),
		"/api/media/generated/images/two.png": []byte("two"),
	}}
	svc := NewService(gen, storage, reviewer)

	result, err := svc.Generate(context.Background(), Request{Description: "Company vision slide"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if result.RetryCount != 1 {
		t.Fatalf("retry_count = %d, want 1", result.RetryCount)
	}
	if result.TaskID != "task-2" {
		t.Fatalf("task_id = %q, want task-2", result.TaskID)
	}
	if result.ReviewScore != 92 {
		t.Fatalf("review_score = %.1f, want 92", result.ReviewScore)
	}
	if len(gen.createReqs) != 2 {
		t.Fatalf("create calls = %d, want 2", len(gen.createReqs))
	}
	if !strings.Contains(gen.createReqs[1].Prompt, "Layout feels crowded") {
		t.Fatalf("retry prompt missing repair delta: %q", gen.createReqs[1].Prompt)
	}
	if result.ImageURLs[0] != "/api/media/generated/images/two.png" {
		t.Fatalf("final image url = %q", result.ImageURLs[0])
	}
	if reviewer.inputs[0].Profile != DefaultQualityProfile {
		t.Fatalf("review profile = %q, want %q", reviewer.inputs[0].Profile, DefaultQualityProfile)
	}
}

func TestServiceKeepsFirstResultWhenReviewFails(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models:       []mediagen.MediaModelInfo{{ID: "nano-banana-pro", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I}},
		tasks: []*mediagen.MediaTask{{
			BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
			Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
		}},
	}
	svc := NewService(gen, fakeStorage{files: map[string][]byte{"/api/media/generated/images/one.png": []byte("one")}}, &fakeReviewer{err: errors.New("vlm down")})

	result, err := svc.Generate(context.Background(), Request{Description: "Roadmap cover"})
	if err != nil {
		t.Fatalf("Generate returned hard error: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected review error summary")
	}
	if result.TaskID != "task-1" {
		t.Fatalf("task_id = %q, want task-1", result.TaskID)
	}
	if len(result.ImageURLs) != 1 {
		t.Fatalf("image_urls = %#v, want first result preserved", result.ImageURLs)
	}
}
