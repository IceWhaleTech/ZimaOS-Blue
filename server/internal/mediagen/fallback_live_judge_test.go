package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	imagedraw "image/draw"
	"image/png"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/videogen"
	xdraw "golang.org/x/image/draw"
)

type mediaJudgeConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Vision   bool
}

type mediaJudgeResult struct {
	Verdict string             `json:"verdict"`
	Score   float64            `json:"score"`
	Reason  string             `json:"reason"`
	Checks  map[string]float64 `json:"checks,omitempty"`
	Risks   []string           `json:"risks,omitempty"`
}

type mediaJudgeReport struct {
	JudgeModel string           `json:"judge_model"`
	JudgeMode  string           `json:"judge_mode"`
	Image      mediaJudgeResult `json:"image"`
	Video      mediaJudgeResult `json:"video"`
}

type compositeMediaJudgeFixture struct {
	Prompt             string
	ImagePNGBase64     string
	ImageSourceURLs    []string
	VideoPreviewBase64 string
	VideoSourceURLs    []string
	VideoSummary       map[string]interface{}
}

func TestFallbackCompositeMediaLLMJudgeOptional(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZIMA_RUN_MEDIA_LLM_JUDGE")) != "1" {
		t.Skip("set ZIMA_RUN_MEDIA_LLM_JUDGE=1 to run the optional composite-media llm judge")
	}

	cfg, err := mediaJudgeConfigFromEnv()
	if err != nil {
		t.Skip(err.Error())
	}

	fixture := buildCompositeMediaJudgeFixture(t)
	judgeMode := "structured"
	if cfg.Vision {
		judgeMode = "vision"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	imageResult, err := runCompositeMediaJudge(ctx, cfg, "image", fixture.Prompt, map[string]interface{}{
		"asset_labels":       []string{"beach", "dog", "cat"},
		"source_asset_count": len(fixture.ImageSourceURLs),
		"composition_summary": map[string]interface{}{
			"background": "beach",
			"subjects": []map[string]string{
				{"label": "dog", "position": "right"},
				{"label": "cat", "position": "left"},
			},
		},
		"notes": []string{
			"Expected two clearly separated foreground subjects.",
			"Expected a beach-like background supporting the prompt.",
		},
	}, fixture.ImagePNGBase64)
	if err != nil {
		t.Fatalf("runCompositeMediaJudge(image): %v", err)
	}

	videoResult, err := runCompositeMediaJudge(ctx, cfg, "video_preview", fixture.Prompt, map[string]interface{}{
		"asset_labels":       []string{"beach", "dog", "cat"},
		"source_asset_count": len(fixture.VideoSourceURLs),
		"composition_summary": map[string]interface{}{
			"background": "beach",
			"subjects": []map[string]string{
				{"label": "dog", "position": "right"},
				{"label": "cat", "position": "left"},
			},
		},
		"video_plan": fixture.VideoSummary,
		"notes": []string{
			"The preview represents the composed thumbnail for a text-to-video fallback plan.",
			"Judge whether the planned composition still preserves both subjects and the intended left/right separation.",
		},
	}, fixture.VideoPreviewBase64)
	if err != nil {
		t.Fatalf("runCompositeMediaJudge(video_preview): %v", err)
	}

	report := mediaJudgeReport{
		JudgeModel: cfg.Model,
		JudgeMode:  judgeMode,
		Image:      imageResult,
		Video:      videoResult,
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(report): %v", err)
	}
	t.Log(string(payload))

	if report.Image.Verdict == "fail" {
		t.Fatalf("image llm judge failed: %+v", report.Image)
	}
	if report.Video.Verdict == "fail" {
		t.Fatalf("video llm judge failed: %+v", report.Video)
	}
}

func mediaJudgeConfigFromEnv() (mediaJudgeConfig, error) {
	cfg := mediaJudgeConfig{
		Provider: strings.TrimSpace(os.Getenv("ZIMA_MEDIA_JUDGE_PROVIDER")),
		BaseURL:  strings.TrimSpace(os.Getenv("ZIMA_MEDIA_JUDGE_BASE_URL")),
		APIKey:   strings.TrimSpace(os.Getenv("ZIMA_MEDIA_JUDGE_API_KEY")),
		Model:    strings.TrimSpace(os.Getenv("ZIMA_MEDIA_JUDGE_MODEL")),
		Vision:   envBool("ZIMA_MEDIA_JUDGE_VISION"),
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return mediaJudgeConfig{}, fmt.Errorf("missing ZIMA_MEDIA_JUDGE_BASE_URL or ZIMA_MEDIA_JUDGE_API_KEY or ZIMA_MEDIA_JUDGE_MODEL")
	}
	if cfg.Provider == "" {
		cfg.Provider = inferMediaJudgeProvider(cfg.Model, cfg.BaseURL)
	}
	return cfg, nil
}

func inferMediaJudgeProvider(model, baseURL string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	baseURL = strings.ToLower(strings.TrimSpace(baseURL))
	if strings.Contains(model, "claude") || strings.Contains(baseURL, "anthropic") {
		return "claude"
	}
	return "openai"
}

func buildCompositeMediaJudgeFixture(t *testing.T) compositeMediaJudgeFixture {
	t.Helper()

	searcher := newCompositeFallbackSearcher(
		newCompositeAssetURL(t, "beach"),
		newCompositeAssetURL(t, "dog"),
		newCompositeAssetURL(t, "cat"),
	)
	engine := NewFallbackEngine(FallbackConfig{
		Enabled: true,
		NativeVideo: FallbackNativeVideoConfig{
			Enabled:            true,
			DefaultDurationSec: 2,
			MaxDurationSec:     2,
			AudioMode:          "silent",
		},
	}, nil, searcher, func() FallbackBrowserService { return nil }, "en-US")

	prompt := "dog on right, cat on left in beach"
	imagePNGBase64, imageSourceURLs, err := engine.renderSceneCompose(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Prompt: prompt,
	})
	if err != nil {
		t.Fatalf("renderSceneCompose returned error: %v", err)
	}

	videoPlan, videoSourceURLs, err := engine.buildNativeVideoPlan(context.Background(), &MediaRequest{
		Type:     MediaTypeVideo,
		Prompt:   prompt,
		Duration: 2,
	}, CategoryT2V, t.TempDir())
	if err != nil {
		t.Fatalf("buildNativeVideoPlan returned error: %v", err)
	}

	videoPreviewBase64, err := renderVideoPlanPreviewPNGBase64(videoPlan)
	if err != nil {
		t.Fatalf("renderVideoPlanPreviewPNGBase64 returned error: %v", err)
	}

	return compositeMediaJudgeFixture{
		Prompt:             prompt,
		ImagePNGBase64:     imagePNGBase64,
		ImageSourceURLs:    append([]string(nil), imageSourceURLs...),
		VideoPreviewBase64: videoPreviewBase64,
		VideoSourceURLs:    append([]string(nil), videoSourceURLs...),
		VideoSummary:       summarizeVideoPlan(videoPlan),
	}
}

func renderVideoPlanPreviewPNGBase64(plan *VideoPlan) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("video plan is required")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, plan.Width, plan.Height))
	type sortableLayer struct {
		ID    string
		Z     int
		Start int
		Layer imageLayer
	}
	drawLayers := make([]sortableLayer, 0, len(plan.Layers))
	for idx, layer := range plan.Layers {
		drawLayers = append(drawLayers, sortableLayer{
			ID:    layer.ID,
			Z:     layer.ZIndex,
			Start: idx,
			Layer: imageLayer{Path: layer.ImagePath, Rect: layer.FrameStart},
		})
	}
	sort.SliceStable(drawLayers, func(i, j int) bool {
		if drawLayers[i].Z == drawLayers[j].Z {
			return drawLayers[i].Start < drawLayers[j].Start
		}
		return drawLayers[i].Z < drawLayers[j].Z
	})

	for _, entry := range drawLayers {
		file, err := os.Open(entry.Layer.Path)
		if err != nil {
			return "", err
		}
		src, _, err := image.Decode(file)
		_ = file.Close()
		if err != nil {
			return "", err
		}
		dest := image.Rect(
			int(math.Round(entry.Layer.Rect.X)),
			int(math.Round(entry.Layer.Rect.Y)),
			int(math.Round(entry.Layer.Rect.X+entry.Layer.Rect.W)),
			int(math.Round(entry.Layer.Rect.Y+entry.Layer.Rect.H)),
		)
		if dest.Empty() {
			continue
		}
		xdraw.CatmullRom.Scale(canvas, dest, src, src.Bounds(), imagedraw.Over, nil)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

type imageLayer struct {
	Path string
	Rect videogen.FrameRect
}

func summarizeVideoPlan(plan *VideoPlan) map[string]interface{} {
	if plan == nil {
		return nil
	}
	layers := make([]map[string]interface{}, 0, len(plan.Layers))
	for _, layer := range plan.Layers {
		layers = append(layers, map[string]interface{}{
			"id":           layer.ID,
			"content_mode": layer.ContentMode,
			"z_index":      layer.ZIndex,
			"frame_start":  layer.FrameStart,
			"frame_end":    layer.FrameEnd,
		})
	}
	return map[string]interface{}{
		"width":        plan.Width,
		"height":       plan.Height,
		"duration_sec": plan.DurationSec,
		"fps":          plan.FPS,
		"layer_count":  len(plan.Layers),
		"layers":       layers,
	}
}

func runCompositeMediaJudge(
	ctx context.Context,
	cfg mediaJudgeConfig,
	mediaType string,
	prompt string,
	evidence map[string]interface{},
	imagePNGBase64 string,
) (mediaJudgeResult, error) {
	provider := newMediaJudgeProvider(cfg)
	if provider == nil {
		return mediaJudgeResult{}, fmt.Errorf("unsupported judge provider %q", cfg.Provider)
	}

	payload := map[string]interface{}{
		"media_type": mediaType,
		"prompt":     strings.TrimSpace(prompt),
		"evidence":   evidence,
		"judge_mode": "structured",
		"expectations": []string{
			"Both required subjects should be present.",
			"The two subjects should not collapse into a single merged subject.",
			"The left-right layout cue should be preserved.",
			"The background should remain relevant to the beach setting.",
		},
	}
	if cfg.Vision && strings.TrimSpace(imagePNGBase64) != "" {
		payload["judge_mode"] = "vision"
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mediaJudgeResult{}, err
	}

	userMessage := mediaJudgeUserMessage(string(body), imagePNGBase64, cfg.Vision, false)
	repairUserMessage := mediaJudgeUserMessage(string(body), imagePNGBase64, cfg.Vision, true)

	systemPrompt := strings.TrimSpace(`You are a strict LLM judge for fallback composite-media outputs.

Return only one JSON object:
{
  "verdict": "pass|partial|fail",
  "score": <number between 0 and 1>,
  "reason": "<brief grounded rationale>",
  "checks": {
    "prompt_alignment": <0..1>,
    "subject_coverage": <0..1>,
    "layout_separation": <0..1>,
    "background_relevance": <0..1>
  },
  "risks": ["..."]
}

Rules:
- If an image is attached, inspect it directly.
- If no image is attached, judge only from the structured evidence and do not invent pixel-level flaws.
- Use "fail" when a required subject, scene, or layout cue is materially missing.
- Use "partial" when the composition is plausible but incomplete or weak.
- Keep the rationale concise and grounded.`)

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Model: cfg.Model,
		Messages: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: systemPrompt,
			},
			userMessage,
		},
		MaxTokens:   700,
		Temperature: 0,
	})
	if err != nil {
		return mediaJudgeResult{}, err
	}

	result, parseErr := parseMediaJudgeResult(resp.Message.Content)
	if parseErr == nil {
		return result, nil
	}

	retryResp, err := provider.Chat(ctx, llm.ChatRequest{
		Model: cfg.Model,
		Messages: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: systemPrompt,
			},
			repairUserMessage,
		},
		MaxTokens:   700,
		Temperature: 0,
	})
	if err != nil {
		return mediaJudgeResult{}, fmt.Errorf("parse judge response: %w raw=%q; retry failed: %v", parseErr, truncateJudgeString(resp.Message.Content, 600), err)
	}
	result, retryParseErr := parseMediaJudgeResult(retryResp.Message.Content)
	if retryParseErr != nil {
		return mediaJudgeResult{}, fmt.Errorf("parse judge response: %w raw=%q; retry parse: %w raw=%q", parseErr, truncateJudgeString(resp.Message.Content, 600), retryParseErr, truncateJudgeString(retryResp.Message.Content, 600))
	}
	return result, nil
}

func mediaJudgeUserMessage(payload, imagePNGBase64 string, vision, forceJSON bool) llm.Message {
	text := payload
	if forceJSON {
		text = "Return only valid JSON that matches the requested schema. Do not add greetings, explanations, markdown, or any extra prose.\n\nJudge payload:\n" + payload
	}

	msg := llm.Message{Role: llm.RoleUser}
	if vision && strings.TrimSpace(imagePNGBase64) != "" {
		msg.ContentParts = []llm.ContentPart{
			{
				Type: "text",
				Text: text,
			},
			{
				Type:      "image",
				MediaType: "image/png",
				Data:      strings.TrimSpace(imagePNGBase64),
			},
		}
		return msg
	}
	msg.Content = text
	return msg
}

func parseMediaJudgeResult(raw string) (mediaJudgeResult, error) {
	var result mediaJudgeResult
	payloadJSON := extractJudgeJSONObject(raw)
	if err := json.Unmarshal([]byte(payloadJSON), &result); err != nil {
		return mediaJudgeResult{}, err
	}
	return result, nil
}

func newMediaJudgeProvider(cfg mediaJudgeConfig) llm.Provider {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "claude", "anthropic":
		return llm.NewClaudeProvider(cfg.APIKey, cfg.BaseURL)
	case "custom":
		return llm.NewCustomProvider(cfg.APIKey, cfg.BaseURL)
	case "openai":
		return llm.NewOpenAIProvider(cfg.APIKey, cfg.BaseURL)
	default:
		return nil
	}
}

func envBool(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}

func extractJudgeJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func truncateJudgeString(raw string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(raw))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}
