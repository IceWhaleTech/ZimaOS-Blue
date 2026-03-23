package mediagen

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scenecompose"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/videogen"
	"github.com/google/uuid"
)

const (
	FallbackModelNativeT2V  = "fallback-native-t2v"
	FallbackModelNativeI2V  = "fallback-native-i2v"
	FallbackModelNativeKF2V = "fallback-native-kf2v"
)

var nativeVideoOSAvailable = func() bool {
	return runtime.GOOS == "darwin"
}

type AudioPlan struct {
	Mode          string
	Narration     string
	EffectiveMode string
	AudioPath     string
}

type VideoPlan struct {
	Category       MediaCategory
	Width          int
	Height         int
	DurationSec    int
	FPS            int
	Layers         []videogen.LayerSpec
	Audio          AudioPlan
	ThumbnailFrame float64
}

func (e *FallbackEngine) SetTTSService(service tts.Service) {
	if e == nil {
		return
	}
	e.ttsService = service
}

func (e *FallbackEngine) SetNativeVideoGenerator(generator videogen.Generator) {
	if e == nil {
		return
	}
	e.nativeVideo = generator
}

func (e *FallbackEngine) nativeVideoAvailable() bool {
	if e == nil || !e.config.NativeVideo.Enabled || e.nativeVideo == nil {
		return false
	}
	if !nativeVideoOSAvailable() {
		return false
	}
	return e.nativeVideo.Available()
}

func (e *FallbackEngine) nativeVideoModelInfos() []MediaModelInfo {
	return []MediaModelInfo{
		{ID: FallbackModelNativeT2V, Name: "Fallback Native Video (Text)", Type: MediaTypeVideo, Category: CategoryT2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyNativeVideo},
		{ID: FallbackModelNativeI2V, Name: "Fallback Native Video (Image)", Type: MediaTypeVideo, Category: CategoryI2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyNativeVideo},
		{ID: FallbackModelNativeKF2V, Name: "Fallback Native Video (Keyframe)", Type: MediaTypeVideo, Category: CategoryKF2V, Provider: fallbackProviderName, IsFallback: true, FallbackStrategy: FallbackStrategyNativeVideo},
	}
}

func isNativeFallbackVideoModel(modelID string) bool {
	switch strings.TrimSpace(modelID) {
	case FallbackModelNativeT2V, FallbackModelNativeI2V, FallbackModelNativeKF2V:
		return true
	default:
		return false
	}
}

func publicSpaceModelForCategory(category MediaCategory) string {
	switch category {
	case CategoryT2V:
		return FallbackModelSpaceT2V
	case CategoryI2V:
		return FallbackModelSpaceI2V
	case CategoryKF2V:
		return FallbackModelSpaceKF2V
	default:
		return ""
	}
}

func (e *FallbackEngine) generateNativeVideo(ctx context.Context, req *MediaRequest, category MediaCategory, modelID string) (*MediaTask, error) {
	if !e.nativeVideoAvailable() {
		if fallbackModel := publicSpaceModelForCategory(category); fallbackModel != "" {
			return e.generatePublicSpace(ctx, req, category, fallbackModel)
		}
		return nil, fmt.Errorf("native video helper unavailable")
	}
	internalID := uuid.New().String()
	task := &MediaTask{
		BaseTask: basetask.BaseTask{
			ID:       internalID,
			Status:   TaskStatusProcessing,
			Progress: 0.05,
		},
		Type:         MediaTypeVideo,
		Category:     string(category),
		Provider:     fallbackProviderName,
		Model:        modelID,
		FallbackInfo: e.PendingInfoForRequest(req, modelID),
	}
	e.tasks.Store(internalID, cloneMediaTask(task))

	go e.runNativeVideoTask(internalID, cloneMediaRequest(req), category, modelID)

	return &MediaTask{
		BaseTask: basetask.BaseTask{
			Status:   TaskStatusProcessing,
			Progress: 0.05,
		},
		Type:         MediaTypeVideo,
		Provider:     fallbackProviderName,
		Model:        modelID,
		UpstreamID:   internalID,
		FallbackInfo: cloneFallbackInfo(task.FallbackInfo),
	}, nil
}

func (e *FallbackEngine) runNativeVideoTask(taskID string, req *MediaRequest, category MediaCategory, modelID string) {
	if e.storage == nil {
		e.failInternalTask(taskID, "native video stage: media storage unavailable")
		return
	}
	workDir, err := os.MkdirTemp("", "mediagen-native-video-*")
	if err != nil {
		e.failInternalTask(taskID, fmt.Sprintf("native video stage: %v", err))
		return
	}
	defer os.RemoveAll(workDir)

	brief := slideBriefFromMediaRequest(req)
	plan, sourceURLs, err := e.buildNativeVideoPlan(context.Background(), req, category, workDir)
	if err != nil {
		e.failInternalTask(taskID, fmt.Sprintf("native video plan stage: %v", err))
		return
	}

	outputPath := filepath.Join(workDir, "output.mp4")
	thumbnailPath := filepath.Join(workDir, "thumbnail.png")
	statusPath := filepath.Join(workDir, "status.json")
	job := &videogen.Job{
		Width:         plan.Width,
		Height:        plan.Height,
		FPS:           plan.FPS,
		DurationSec:   plan.DurationSec,
		Layers:        append([]videogen.LayerSpec(nil), plan.Layers...),
		AudioPath:     plan.Audio.AudioPath,
		OutputPath:    outputPath,
		ThumbnailPath: thumbnailPath,
		StatusPath:    statusPath,
		ThumbnailSec:  plan.ThumbnailFrame,
	}

	e.updateInternalTask(taskID, func(task *MediaTask) {
		task.Progress = 0.1
		task.FallbackInfo = e.newFallbackInfo(FallbackStrategyNativeVideo, fallbackDisplayName(FallbackStrategyNativeVideo), sourceURLs, nil, "", brief, brief.TemplateID)
	})

	result, err := e.nativeVideo.Generate(
		context.Background(),
		job,
		e.config.NativeVideo.PollInterval,
		e.config.NativeVideo.StallTimeout,
		e.config.NativeVideo.MaxRuntime,
		func(progress videogen.Progress) {
			e.updateInternalTask(taskID, func(task *MediaTask) {
				task.Status = TaskStatusProcessing
				if progress.Progress > task.Progress {
					task.Progress = progress.Progress
				}
			})
		},
	)
	if err != nil {
		e.failInternalTask(taskID, fmt.Sprintf("native video render stage: %v", err))
		return
	}

	videoBytes, err := os.ReadFile(result.OutputPath)
	if err != nil {
		e.failInternalTask(taskID, fmt.Sprintf("native video result stage: %v", err))
		return
	}
	videoURL, err := e.storage.StoreBytes(videoBytes, "video/mp4", MediaTypeVideo)
	if err != nil {
		e.failInternalTask(taskID, fmt.Sprintf("native video store stage: %v", err))
		return
	}

	thumbnailURL := ""
	if thumbBytes, thumbErr := os.ReadFile(result.ThumbnailPath); thumbErr == nil && len(thumbBytes) > 0 {
		thumbnailURL, _ = e.storage.StoreThumbnailBytes(thumbBytes, "image/png")
	}

	e.updateInternalTask(taskID, func(task *MediaTask) {
		task.Status = TaskStatusSucceeded
		task.Progress = 1
		task.Response = &MediaResponse{
			Created: timeutil.NowTime().Unix(),
			Data: []MediaResult{{
				URL:          videoURL,
				ThumbnailURL: thumbnailURL,
				ContentType:  "video/mp4",
				Width:        plan.Width,
				Height:       plan.Height,
				DurationSec:  plan.DurationSec,
			}},
		}
		task.FallbackInfo = e.newFallbackInfo(FallbackStrategyNativeVideo, fallbackDisplayName(FallbackStrategyNativeVideo), sourceURLs, nil, "", brief, brief.TemplateID)
	})
}

func (e *FallbackEngine) buildNativeVideoPlan(ctx context.Context, req *MediaRequest, category MediaCategory, workDir string) (*VideoPlan, []string, error) {
	width, height := nativeVideoCanvas(req)
	durationSec := e.nativeVideoDuration(req)
	plan := &VideoPlan{
		Category:       category,
		Width:          width,
		Height:         height,
		DurationSec:    durationSec,
		FPS:            e.config.NativeVideo.FPS,
		ThumbnailFrame: math.Max(0.25, float64(durationSec)*0.5),
	}

	var sourceURLs []string
	var scenePlan *scenecompose.ScenePlan
	switch category {
	case CategoryT2V:
		sceneLayers, planRef, urls, err := e.buildNativeT2VLayers(ctx, req, workDir, width, height, durationSec)
		if err != nil {
			return nil, nil, err
		}
		plan.Layers = sceneLayers
		scenePlan = planRef
		sourceURLs = urls
	case CategoryI2V:
		layers, urls, err := e.buildNativeI2VLayers(ctx, req, workDir, width, height, durationSec)
		if err != nil {
			return nil, nil, err
		}
		plan.Layers = layers
		sourceURLs = urls
	case CategoryKF2V:
		layers, urls, err := e.buildNativeKF2VLayers(ctx, req, workDir, width, height, durationSec)
		if err != nil {
			return nil, nil, err
		}
		plan.Layers = layers
		sourceURLs = urls
	default:
		return nil, nil, fmt.Errorf("unsupported native video category %s", category)
	}

	audioPlan := e.buildAudioPlan(req, category, scenePlan)
	if audioPath, err := e.materializeAudioPlan(ctx, workDir, &audioPlan); err != nil {
		return nil, nil, err
	} else {
		audioPlan.AudioPath = audioPath
	}
	plan.Audio = audioPlan
	return plan, sourceURLs, nil
}

func (e *FallbackEngine) buildNativeT2VLayers(ctx context.Context, req *MediaRequest, workDir string, width, height, durationSec int) ([]videogen.LayerSpec, *scenecompose.ScenePlan, []string, error) {
	if e.sceneComposer != nil {
		result, err := e.sceneComposer.Compose(ctx, scenecompose.ComposeRequest{
			Prompt: reqPrompt(req),
			Width:  width,
			Height: height,
			Locale: e.locale,
		})
		if err == nil && result != nil && len(result.Layers) > 0 {
			layers, layerErr := e.materializeSceneComposeLayers(result, workDir, width, height, durationSec)
			if layerErr == nil && len(layers) > 0 {
				return layers, result.Debug.Plan, assetRefsToSourceURLs(result.UsedAssets), nil
			}
		}
	}

	imagePath, urls, err := e.buildNativeStaticFallbackImage(ctx, req, workDir)
	if err != nil {
		return nil, nil, nil, err
	}
	layers := []videogen.LayerSpec{
		buildBackgroundLayer("fallback-static", imagePath, width, height, float64(durationSec)),
	}
	return layers, nil, urls, nil
}

func (e *FallbackEngine) buildNativeI2VLayers(ctx context.Context, req *MediaRequest, workDir string, width, height, durationSec int) ([]videogen.LayerSpec, []string, error) {
	files, cleanup, err := e.prepareReferenceFiles(ctx, req)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("image-to-video requires a reference image")
	}
	refPath := files[0]
	sourceURLs := nativeReferenceSourceURLs(req, 1)

	layers := []videogen.LayerSpec{
		buildBackgroundLayer("i2v-base", refPath, width, height, float64(durationSec)),
	}
	if e.u2netpModel != nil {
		cutPath, err := e.buildCutoutLayerImage(ctx, refPath, workDir)
		if err == nil && cutPath != "" {
			layers = append(layers, buildFullFrameForegroundLayer("i2v-subject", cutPath, width, height, float64(durationSec)))
		}
	}
	if len(layers) == 1 {
		layers[0] = buildKenBurnsLayer("i2v-kenburns", refPath, width, height, float64(durationSec), 0.04)
	}
	return layers, sourceURLs, nil
}

func (e *FallbackEngine) buildNativeKF2VLayers(ctx context.Context, req *MediaRequest, _ string, width, height, durationSec int) ([]videogen.LayerSpec, []string, error) {
	files, cleanup, err := e.prepareReferenceFiles(ctx, req)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, nil, err
	}
	if len(files) < 2 {
		return nil, nil, fmt.Errorf("keyframe video requires two reference images")
	}

	total := float64(durationSec)
	crossfadeStart := total * 0.45
	crossfadeEnd := total * 0.55
	layers := []videogen.LayerSpec{
		{
			ID:           "kf2v-start",
			ImagePath:    files[0],
			ContentMode:  "fill",
			ZIndex:       0,
			StartSec:     0,
			EndSec:       crossfadeEnd,
			FrameStart:   fullFrameRect(width, height, 1.00, -0.012*float64(width), -0.008*float64(height)),
			FrameEnd:     fullFrameRect(width, height, 1.05, 0.012*float64(width), 0.008*float64(height)),
			OpacityStart: 1,
			OpacityEnd:   0,
		},
		{
			ID:           "kf2v-end",
			ImagePath:    files[1],
			ContentMode:  "fill",
			ZIndex:       1,
			StartSec:     crossfadeStart,
			EndSec:       total,
			FrameStart:   fullFrameRect(width, height, 1.02, 0.014*float64(width), 0.010*float64(height)),
			FrameEnd:     fullFrameRect(width, height, 1.06, -0.014*float64(width), -0.010*float64(height)),
			OpacityStart: 0,
			OpacityEnd:   1,
		},
	}
	return layers, nativeReferenceSourceURLs(req, 2), nil
}

func (e *FallbackEngine) materializeSceneComposeLayers(result *scenecompose.ComposeResult, workDir string, width, height, durationSec int) ([]videogen.LayerSpec, error) {
	if result == nil || len(result.Layers) == 0 {
		return nil, fmt.Errorf("scene compose returned no reusable layers")
	}
	layers := make([]videogen.LayerSpec, 0, len(result.Layers))
	foregroundIndex := 0
	for _, layer := range result.Layers {
		if layer.Image == nil {
			continue
		}
		imagePath, err := writeImageToPNGFile(workDir, layer.ID, layer.Image)
		if err != nil {
			return nil, err
		}
		switch layer.Kind {
		case "background":
			layers = append(layers, buildBackgroundLayer(firstNonEmptyValue(layer.ID, "background"), imagePath, width, height, float64(durationSec)))
		case "foreground":
			placement := scenecompose.LayoutPlacement(width, height, layer.Layout, foregroundIndex)
			layers = append(layers, buildForegroundLayer(firstNonEmptyValue(layer.ID, fmt.Sprintf("foreground-%d", foregroundIndex+1)), imagePath, layer.Layout, placement, width, height, float64(durationSec)))
			foregroundIndex++
		}
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("scene compose layer materialization produced no layers")
	}
	return layers, nil
}

func buildBackgroundLayer(id, imagePath string, width, height int, durationSec float64) videogen.LayerSpec {
	return buildKenBurnsLayer(id, imagePath, width, height, durationSec, 0.06)
}

func buildKenBurnsLayer(id, imagePath string, width, height int, durationSec float64, zoomDelta float64) videogen.LayerSpec {
	return videogen.LayerSpec{
		ID:           id,
		ImagePath:    imagePath,
		ContentMode:  "fill",
		ZIndex:       0,
		StartSec:     0,
		EndSec:       durationSec,
		FrameStart:   fullFrameRect(width, height, 1.00, -0.012*float64(width), -0.008*float64(height)),
		FrameEnd:     fullFrameRect(width, height, 1.00+zoomDelta, 0.012*float64(width), 0.008*float64(height)),
		OpacityStart: 1,
		OpacityEnd:   1,
	}
}

func buildFullFrameForegroundLayer(id, imagePath string, width, height int, durationSec float64) videogen.LayerSpec {
	return videogen.LayerSpec{
		ID:           id,
		ImagePath:    imagePath,
		ContentMode:  "fill",
		ZIndex:       10,
		StartSec:     0,
		EndSec:       durationSec,
		FrameStart:   fullFrameRect(width, height, 1.00, 0, 0),
		FrameEnd:     fullFrameRect(width, height, 1.03, 0.010*float64(width), -0.004*float64(height)),
		OpacityStart: 1,
		OpacityEnd:   1,
	}
}

func buildForegroundLayer(id, imagePath string, layout scenecompose.LayoutHint, placement scenecompose.Placement, width, height int, durationSec float64) videogen.LayerSpec {
	start := rectFromPlacement(placement)
	driftX := 0.02 * float64(width)
	switch strings.ToLower(strings.TrimSpace(layout.Horizontal)) {
	case "left":
		driftX = 0.03 * float64(width)
	case "right":
		driftX = -0.03 * float64(width)
	default:
		driftX = 0.01 * float64(width)
	}
	driftY := 0.02 * float64(height)
	switch strings.ToLower(strings.TrimSpace(layout.Vertical)) {
	case "high":
		driftY = 0.012 * float64(height)
	case "low":
		driftY = -0.012 * float64(height)
	default:
		driftY = 0.006 * float64(height)
	}
	scaleDelta := 0.02
	if strings.EqualFold(strings.TrimSpace(layout.Depth), "foreground") {
		scaleDelta = 0.04
	}
	end := moveAndScaleRect(start, driftX, driftY, 1+scaleDelta)
	return videogen.LayerSpec{
		ID:           id,
		ImagePath:    imagePath,
		ContentMode:  "fit",
		ZIndex:       10,
		StartSec:     0,
		EndSec:       durationSec,
		FrameStart:   start,
		FrameEnd:     end,
		OpacityStart: 1,
		OpacityEnd:   1,
	}
}

func rectFromPlacement(pos scenecompose.Placement) videogen.FrameRect {
	return videogen.FrameRect{
		X: float64(pos.X - pos.W/2),
		Y: float64(pos.GroundY - pos.H),
		W: float64(pos.W),
		H: float64(pos.H),
	}
}

func moveAndScaleRect(rect videogen.FrameRect, dx, dy, scale float64) videogen.FrameRect {
	cx := rect.X + rect.W/2 + dx
	cy := rect.Y + rect.H/2 + dy
	w := rect.W * scale
	h := rect.H * scale
	return videogen.FrameRect{
		X: cx - w/2,
		Y: cy - h/2,
		W: w,
		H: h,
	}
}

func fullFrameRect(width, height int, scale, shiftX, shiftY float64) videogen.FrameRect {
	w := float64(width) * scale
	h := float64(height) * scale
	return videogen.FrameRect{
		X: (float64(width)-w)/2 + shiftX,
		Y: (float64(height)-h)/2 + shiftY,
		W: w,
		H: h,
	}
}

func (e *FallbackEngine) buildNativeStaticFallbackImage(ctx context.Context, req *MediaRequest, workDir string) (string, []string, error) {
	imageReq := cloneMediaRequest(req)
	imageReq.Type = MediaTypeImage
	task, err := e.generateWebCanvas(ctx, imageReq)
	if err != nil {
		return "", nil, err
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		return "", nil, fmt.Errorf("static fallback render returned no image")
	}
	payload, err := e.readImageResultBytes(ctx, task.Response.Data[0])
	if err != nil {
		return "", nil, err
	}
	imagePath, err := writeReferenceTempFile(workDir, payload, "image/png")
	if err != nil {
		return "", nil, err
	}
	var sourceURLs []string
	if task.FallbackInfo != nil {
		sourceURLs = append(sourceURLs, task.FallbackInfo.SourceURLs...)
	}
	return imagePath, sourceURLs, nil
}

func (e *FallbackEngine) readImageResultBytes(ctx context.Context, result MediaResult) ([]byte, error) {
	if strings.TrimSpace(result.B64JSON) != "" {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(result.B64JSON))
	}
	if strings.TrimSpace(result.URL) != "" {
		contentType, payload, err := e.readReferencePayload(ctx, result.URL)
		if err != nil {
			return nil, err
		}
		if contentType == "" {
			contentType = http.DetectContentType(payload)
		}
		if !strings.HasPrefix(contentType, "image/") {
			return nil, fmt.Errorf("result is not an image")
		}
		return payload, nil
	}
	return nil, fmt.Errorf("result contains no image payload")
}

func writeImageToPNGFile(dir, prefix string, img image.Image) (string, error) {
	name := strings.TrimSpace(prefix)
	if name == "" {
		name = uuid.New().String()
	}
	path := filepath.Join(dir, name+".png")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return "", err
	}
	return path, nil
}

func (e *FallbackEngine) buildCutoutLayerImage(ctx context.Context, refPath string, workDir string) (string, error) {
	payload, err := os.ReadFile(refPath)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	resolved := &scenecompose.ResolvedImage{
		Title:    filepath.Base(refPath),
		Image:    img,
		Width:    img.Bounds().Dx(),
		Height:   img.Bounds().Dy(),
		HasAlpha: fallbackImageHasAlpha(img),
	}
	cutout := scenecompose.NewCutoutStrategy(e.u2netpModel)
	result, err := cutout.Apply(ctx, resolved)
	if err != nil || result == nil || result.Image == nil || result.Mask == nil {
		return "", fmt.Errorf("cutout unavailable")
	}
	return writeImageToPNGFile(workDir, "cutout-"+uuid.New().String(), result.Image)
}

func nativeReferenceSourceURLs(req *MediaRequest, max int) []string {
	if req == nil {
		return nil
	}
	out := make([]string, 0, max)
	seen := map[string]struct{}{}
	for _, item := range append([]string{req.ReferenceURL}, req.ReferenceURLs...) {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || strings.HasPrefix(trimmed, "data:") {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func nativeVideoCanvas(req *MediaRequest) (int, int) {
	if req != nil {
		if width, height, ok := parseMediaSize(strings.TrimSpace(req.Size)); ok {
			return evenInt(width), evenInt(height)
		}
		if ratio := strings.TrimSpace(mediaRequestExtraString(req, "aspect_ratio", "aspectRatio")); ratio != "" {
			if width, height, ok := canvasForAspectRatio(ratio); ok {
				return evenInt(width), evenInt(height)
			}
		}
	}
	return 1280, 720
}

func parseMediaSize(raw string) (int, int, bool) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == 'x' || r == 'X' || r == '*' })
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func canvasForAspectRatio(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	left, errL := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	right, errR := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errL != nil || errR != nil || left <= 0 || right <= 0 {
		return 0, 0, false
	}
	ratio := left / right
	switch {
	case ratio >= 1:
		return 1280, int(math.Round(1280 / ratio)), true
	default:
		return int(math.Round(720 * ratio)), 720, true
	}
}

func evenInt(value int) int {
	if value <= 0 {
		return 2
	}
	if value%2 == 1 {
		return value + 1
	}
	return value
}

func (e *FallbackEngine) nativeVideoDuration(req *MediaRequest) int {
	duration := e.config.NativeVideo.DefaultDurationSec
	if req != nil && req.Duration > 0 {
		duration = req.Duration
	}
	if duration <= 0 {
		duration = 5
	}
	if limit := e.config.NativeVideo.MaxDurationSec; limit > 0 && duration > limit {
		duration = limit
	}
	return duration
}

func normalizeAudioMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "silent":
		return "silent"
	case "tts":
		return "tts"
	default:
		return "auto"
	}
}

func (e *FallbackEngine) buildAudioPlan(req *MediaRequest, category MediaCategory, scenePlan *scenecompose.ScenePlan) AudioPlan {
	mode := normalizeAudioMode(mediaRequestExtraString(req, "audio_mode", "audioMode"))
	if mode == "auto" {
		mode = normalizeAudioMode(e.config.NativeVideo.AudioMode)
	}
	plan := AudioPlan{
		Mode:          mode,
		EffectiveMode: "silent",
		Narration:     strings.TrimSpace(mediaRequestExtraString(req, "narration")),
	}
	if plan.Mode == "silent" {
		return plan
	}
	if plan.Narration == "" {
		plan.Narration = autoNarrationForRequest(req, category, scenePlan)
	}
	if plan.Narration != "" {
		plan.EffectiveMode = "tts"
	}
	return plan
}

func autoNarrationForRequest(req *MediaRequest, category MediaCategory, scenePlan *scenecompose.ScenePlan) string {
	if scenePlan != nil {
		subjects := make([]string, 0, len(scenePlan.Foreground))
		for _, item := range scenePlan.Foreground {
			if trimmed := strings.TrimSpace(item.Type); trimmed != "" {
				subjects = append(subjects, trimmed)
			}
		}
		if len(subjects) > 0 {
			background := strings.TrimSpace(scenePlan.Background)
			if containsChinese(strings.TrimSpace(firstNonEmptyValue(reqPrompt(req), background))) {
				return strings.TrimSpace(fmt.Sprintf("镜头缓慢推进，%s出现在%s。", strings.Join(subjects, "和"), firstNonEmptyValue(background, "场景中")))
			}
			return strings.TrimSpace(fmt.Sprintf("A gentle camera move reveals %s in %s.", strings.Join(subjects, " and "), firstNonEmptyValue(background, "the scene")))
		}
	}
	prompt := strings.TrimSpace(reqPrompt(req))
	if prompt == "" {
		switch category {
		case CategoryI2V:
			return "A still image slowly comes to life."
		case CategoryKF2V:
			return "A visual transition unfolds between two keyframes."
		default:
			return "A short cinematic scene comes into view."
		}
	}
	if containsChinese(prompt) {
		return truncateSentence(prompt, 28)
	}
	return truncateSentence(prompt, 96)
}

func reqPrompt(req *MediaRequest) string {
	if req == nil {
		return ""
	}
	return normalizeMediaPrompt(req.Prompt)
}

func containsChinese(text string) bool {
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func truncateSentence(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func (e *FallbackEngine) materializeAudioPlan(ctx context.Context, workDir string, plan *AudioPlan) (string, error) {
	if plan == nil || plan.EffectiveMode != "tts" || strings.TrimSpace(plan.Narration) == "" || e.ttsService == nil {
		if plan != nil {
			plan.EffectiveMode = "silent"
		}
		return "", nil
	}

	resp, err := e.ttsService.Synthesize(ctx, &tts.SynthesizeRequest{
		Text:   plan.Narration,
		Format: tts.FormatWAV,
		Speed:  1.0,
		Volume: 100,
	})
	if err != nil {
		plan.EffectiveMode = "silent"
		return "", nil
	}
	if resp == nil || resp.Audio == nil {
		plan.EffectiveMode = "silent"
		return "", nil
	}
	defer resp.Audio.Close()

	audioBytes, err := io.ReadAll(resp.Audio)
	if err != nil || len(audioBytes) == 0 {
		plan.EffectiveMode = "silent"
		return "", nil
	}

	contentType := strings.TrimSpace(resp.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(audioBytes)
	}
	audioPath, err := writeReferenceTempFile(workDir, audioBytes, contentType)
	if err != nil {
		plan.EffectiveMode = "silent"
		return "", nil
	}
	plan.EffectiveMode = "tts"
	return audioPath, nil
}
