package ppt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/slidespec"
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

type fakeLayoutPlanner struct {
	spec     *slidespec.LayoutSpec
	err      error
	requests []LayoutPlanRequest
}

func (f *fakeLayoutPlanner) Plan(_ context.Context, req LayoutPlanRequest) (*slidespec.LayoutSpec, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.spec == nil {
		return nil, nil
	}
	spec := *f.spec
	spec.Elements = append([]slidespec.LayoutElement(nil), f.spec.Elements...)
	return &spec, nil
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

func TestBuildPromptIncludesStructuredSlideBrief(t *testing.T) {
	prompt := buildPrompt(Request{
		Description:    "帮我做一页PPT，标题：2026 产品战略；副标题：AI 驱动增长；要点：提升转化率；降低成本",
		AspectRatio:    "16:9",
		StylePreset:    DefaultStylePreset,
		QualityProfile: DefaultQualityProfile,
		Source:         "ppt",
	}, ModeTextOnly, "")

	if !strings.Contains(prompt, "Structured slide brief:") {
		t.Fatalf("prompt missing structured brief: %q", prompt)
	}
	if !strings.Contains(prompt, "template_id: split") {
		t.Fatalf("prompt missing template_id: %q", prompt)
	}
	if !strings.Contains(prompt, "title: 2026 产品战略") {
		t.Fatalf("prompt missing title: %q", prompt)
	}
	if !strings.Contains(prompt, "visual_query:") {
		t.Fatalf("prompt missing visual_query: %q", prompt)
	}
}

func TestBuildPromptUsesNanoSlidesConstraints(t *testing.T) {
	prompt := buildPrompt(Request{
		Description:    "Create a strategy summary slide",
		AspectRatio:    "16:9",
		StylePreset:    "nano slides",
		QualityProfile: DefaultQualityProfile,
		Source:         "ppt",
	}, ModeTextOnly, "")

	if !strings.Contains(prompt, "nanoslides style constraints:") {
		t.Fatalf("prompt missing nano title: %q", prompt)
	}
	if !strings.Contains(prompt, "nanoslides_signature") {
		t.Fatalf("prompt missing nano signature: %q", prompt)
	}
}

func TestServicePassesThroughExplicitLayoutSpec(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models: []mediagen.MediaModelInfo{
			{ID: "nano-banana-pro", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I},
		},
		tasks: []*mediagen.MediaTask{{
			BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
			Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
		}},
	}
	layout := map[string]any{
		"template_id": "text_only",
		"elements": []map[string]any{
			{
				"kind":      "text",
				"text":      "Revenue growth",
				"font_role": "title",
				"x":         96,
				"y":         120,
				"width":     420,
				"height":    96,
			},
		},
	}
	svc := NewService(gen, fakeStorage{files: map[string][]byte{"/api/media/generated/images/one.png": []byte("png")}}, &fakeReviewer{})

	_, err := svc.Generate(context.Background(), Request{
		Description: "Quarterly growth overview",
		LayoutSpec:  layout,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(gen.createReqs) != 1 {
		t.Fatalf("create calls = %d, want 1", len(gen.createReqs))
	}
	raw, ok := gen.createReqs[0].Extra["layout_spec"]
	if !ok {
		t.Fatal("expected layout_spec in media request extras")
	}
	spec, ok := slidespec.DecodeLayoutSpec(raw)
	if !ok {
		t.Fatalf("layout_spec = %#v, want decodable slidespec.LayoutSpec", raw)
	}
	if spec.TemplateID != slidespec.TemplateTextOnly {
		t.Fatalf("template_id = %q, want %q", spec.TemplateID, slidespec.TemplateTextOnly)
	}
}

func TestServiceBuildsStructuredLayoutSpecWhenImplicit(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models: []mediagen.MediaModelInfo{
			{ID: "nano-banana-pro", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I},
		},
		tasks: []*mediagen.MediaTask{{
			BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
			Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
		}},
	}
	svc := NewService(gen, fakeStorage{files: map[string][]byte{"/api/media/generated/images/one.png": []byte("png")}}, &fakeReviewer{})

	_, err := svc.Generate(context.Background(), Request{
		Description: "Quarterly growth overview",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(gen.createReqs) != 1 {
		t.Fatalf("create calls = %d, want 1", len(gen.createReqs))
	}
	raw, ok := gen.createReqs[0].Extra["layout_spec"]
	if !ok {
		t.Fatal("expected implicit layout_spec in media request extras")
	}
	spec, ok := slidespec.DecodeLayoutSpec(raw)
	if !ok {
		t.Fatalf("layout_spec = %#v, want decodable slidespec.LayoutSpec", raw)
	}
	if spec.Canvas.AspectRatio != DefaultAspectRatio {
		t.Fatalf("aspect_ratio = %q, want %q", spec.Canvas.AspectRatio, DefaultAspectRatio)
	}
	if len(spec.Elements) == 0 {
		t.Fatalf("elements = %#v, want non-empty layout", spec.Elements)
	}
}

func TestServiceUsesPlannerProvidedLayoutSpec(t *testing.T) {
	gen := &fakeGenerator{
		hasProviders: true,
		models: []mediagen.MediaModelInfo{
			{ID: "nano-banana-pro", Type: mediagen.MediaTypeImage, Category: mediagen.CategoryT2I},
		},
		tasks: []*mediagen.MediaTask{{
			BaseTask: basetask.BaseTask{ID: "task-1", Status: mediagen.TaskStatusSucceeded},
			Response: &mediagen.MediaResponse{Data: []mediagen.MediaResult{{URL: "/api/media/generated/images/one.png"}}},
		}},
	}
	planner := &fakeLayoutPlanner{spec: &slidespec.LayoutSpec{
		RenderMode:  slidespec.RenderModeSlide,
		StylePreset: slidespec.StylePresetNanoSlides,
		TemplateID:  slidespec.TemplateCover,
		Theme:       "Aurora brand",
		Canvas: slidespec.LayoutCanvas{
			Width:       1280,
			Height:      720,
			AspectRatio: "16:9",
		},
		Elements: []slidespec.LayoutElement{
			{
				Kind:       "text",
				Name:       "title",
				X:          96,
				Y:          108,
				Width:      420,
				Height:     120,
				ZIndex:     2,
				Text:       "Planner title",
				FontRole:   "title",
				FontSize:   54,
				FontWeight: "bold",
				Color:      "#F8FAFC",
				MaxLines:   2,
			},
		},
	}}
	svc := NewService(gen, fakeStorage{files: map[string][]byte{"/api/media/generated/images/one.png": []byte("png")}}, &fakeReviewer{})
	svc.SetLayoutPlanner(planner)

	_, err := svc.Generate(context.Background(), Request{
		Description: "Quarterly growth overview",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(planner.requests) != 1 {
		t.Fatalf("planner calls = %d, want 1", len(planner.requests))
	}
	raw, ok := gen.createReqs[0].Extra["layout_spec"]
	if !ok {
		t.Fatal("expected planner layout_spec in media request extras")
	}
	spec, ok := slidespec.DecodeLayoutSpec(raw)
	if !ok {
		t.Fatalf("layout_spec = %#v, want decodable slidespec.LayoutSpec", raw)
	}
	if spec.TemplateID != slidespec.TemplateCover {
		t.Fatalf("template_id = %q, want %q", spec.TemplateID, slidespec.TemplateCover)
	}
	if got := gen.createReqs[0].Extra["theme"]; got != "Aurora brand" {
		t.Fatalf("theme = %v, want %q", got, "Aurora brand")
	}
	if !strings.Contains(gen.createReqs[0].Prompt, "template_id: cover") {
		t.Fatalf("prompt missing planner template: %q", gen.createReqs[0].Prompt)
	}
}
