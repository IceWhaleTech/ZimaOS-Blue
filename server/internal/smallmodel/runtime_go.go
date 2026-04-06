//go:build linux && cgo

package smallmodel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	goMaxPromptTokens = 768
)

// GoRuntimeOptions controls pure-Go ONNX runtime behavior.
type GoRuntimeOptions struct {
	Timeout     time.Duration
	MaxParallel int
}

// GoRuntime executes Qwen3.5 ONNX inference directly via onnxruntime_go.
type GoRuntime struct {
	manager *Manager
	timeout time.Duration

	parallelSem chan struct{}

	loadMu       sync.Mutex
	engine       *goEngine
	lastLoadErr  error
	lastLoadCode string
	lastLoadAt   time.Time
}

type goEngine struct {
	modelDir string

	tokenizer *bpeTokenizer

	imageTokenID       int64
	visionStartTokenID int64
	visionEndTokenID   int64
	stopTokenIDs       map[int64]struct{}

	embedSession   *onnx.DynamicSession
	decoderSession *onnx.DynamicSession
	visionSession  *onnx.DynamicSession
}

type modelConfig struct {
	ImageTokenID       int64 `json:"image_token_id"`
	VisionStartTokenID int64 `json:"vision_start_token_id"`
	VisionEndTokenID   int64 `json:"vision_end_token_id"`
	EOSTokenID         int64 `json:"eos_token_id"`
	TextConfig         struct {
		EOSTokenID int64 `json:"eos_token_id"`
	} `json:"text_config"`
}

func NewGoRuntime(manager *Manager, opts ...GoRuntimeOptions) *GoRuntime {
	var opt GoRuntimeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxParallel := opt.MaxParallel
	if maxParallel <= 0 {
		maxParallel = defaultMaxParallel
	}
	return &GoRuntime{
		manager:     manager,
		timeout:     timeout,
		parallelSem: make(chan struct{}, maxParallel),
	}
}

func (r *GoRuntime) Ready() bool {
	reason, _ := r.readinessState()
	return reason == "ready"
}

func (r *GoRuntime) ReadinessReason() string {
	reason, _ := r.readinessState()
	return reason
}

func (r *GoRuntime) ReadinessDetail() string {
	_, detail := r.readinessState()
	return detail
}

func (r *GoRuntime) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	engine, err := r.ensureEngineLoaded()
	if err != nil {
		return nil, ErrNotReady
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("empty prompt")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens > 1024 {
		maxTokens = 1024
	}

	runCtx := ctx
	cancel := func() {}
	if _, ok := runCtx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(runCtx, r.timeout)
	}
	defer cancel()

	select {
	case r.parallelSem <- struct{}{}:
		defer func() { <-r.parallelSem }()
	case <-runCtx.Done():
		return nil, runCtx.Err()
	}

	text, genErr := engine.generate(runCtx, prompt, maxTokens, req.Images)
	if genErr != nil {
		return nil, genErr
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty output from go onnx runtime")
	}
	return &GenerateResponse{Text: strings.TrimSpace(text)}, nil
}

func (r *GoRuntime) readinessState() (string, string) {
	if r == nil {
		return "runtime_nil", "small model runtime is nil"
	}
	if r.manager == nil {
		return "manager_nil", "small model manager is nil"
	}
	st := r.manager.GetStatus()
	if !st.Ready {
		if st.Downloading {
			return "model_downloading", "small model files are downloading"
		}
		if strings.TrimSpace(st.Error) != "" {
			return "model_unready", st.Error
		}
		return "model_files_missing_or_incomplete", "required small model files are missing"
	}
	if _, err := r.ensureEngineLoaded(); err != nil {
		r.loadMu.Lock()
		code := strings.TrimSpace(r.lastLoadCode)
		detail := ""
		if r.lastLoadErr != nil {
			detail = r.lastLoadErr.Error()
		}
		r.loadMu.Unlock()
		if code == "" {
			code = "onnx_runtime_unavailable"
		}
		if detail == "" {
			detail = "failed to initialize go onnx runtime"
		}
		return code, detail
	}
	return "ready", ""
}

func (r *GoRuntime) ensureEngineLoaded() (*goEngine, error) {
	if r == nil {
		return nil, fmt.Errorf("nil runtime")
	}

	r.loadMu.Lock()
	defer r.loadMu.Unlock()

	if r.engine != nil {
		return r.engine, nil
	}

	// Avoid retry storms under frequent readiness checks.
	if r.lastLoadErr != nil && !r.lastLoadAt.IsZero() && time.Since(r.lastLoadAt) < 15*time.Second {
		return nil, r.lastLoadErr
	}

	engine, code, err := loadGoEngine(r.manager)
	if err != nil {
		r.lastLoadErr = err
		r.lastLoadCode = code
		r.lastLoadAt = time.Now()
		return nil, err
	}

	r.engine = engine
	r.lastLoadErr = nil
	r.lastLoadCode = ""
	r.lastLoadAt = time.Time{}
	return r.engine, nil
}

func loadGoEngine(manager *Manager) (*goEngine, string, error) {
	if manager == nil {
		return nil, "manager_nil", fmt.Errorf("nil manager")
	}

	modelDir := manager.ModelDir()
	dataDir := filepath.Dir(filepath.Dir(modelDir))

	onnx.SetDataDir(dataDir)
	libPath := onnx.RuntimeLibPath(dataDir)
	if libPath == "" {
		var err error
		libPath, err = onnx.EnsureRuntime(dataDir)
		if err != nil {
			return nil, "onnx_runtime_library_unavailable", fmt.Errorf("ensure onnxruntime library: %w", err)
		}
	}
	onnx.SetLibraryPath(libPath)

	tok, err := loadTokenizer(filepath.Join(modelDir, "tokenizer.json"))
	if err != nil {
		return nil, "tokenizer_load_failed", err
	}

	cfgData, err := loadModelConfig(filepath.Join(modelDir, "config.json"))
	if err != nil {
		return nil, "model_config_load_failed", err
	}

	embedSession, err := onnx.NewDynamicSession(
		filepath.Join(modelDir, "onnx", "embed_tokens_q4.onnx"),
		[]string{"input_ids"},
		[]string{"inputs_embeds"},
	)
	if err != nil {
		return nil, "embed_session_create_failed", fmt.Errorf("create embed session: %w", err)
	}

	decoderSession, err := onnx.NewDynamicSession(
		filepath.Join(modelDir, "onnx", "decoder_model_merged_q4.onnx"),
		decoderInputNames(),
		decoderOutputNames(),
	)
	if err != nil {
		_ = embedSession.Close()
		return nil, "decoder_session_create_failed", fmt.Errorf("create decoder session: %w", err)
	}

	visionSession, err := onnx.NewDynamicSession(
		filepath.Join(modelDir, "onnx", "vision_encoder_q4.onnx"),
		[]string{"pixel_values", "image_grid_thw"},
		[]string{"image_features"},
	)
	if err != nil {
		_ = decoderSession.Close()
		_ = embedSession.Close()
		return nil, "vision_session_create_failed", fmt.Errorf("create vision session: %w", err)
	}

	stopIDs := map[int64]struct{}{}
	for _, tokName := range []string{"<|endoftext|>", "<|im_end|>"} {
		if id, ok := tok.specialID(tokName); ok {
			stopIDs[id] = struct{}{}
		}
	}
	if cfgData.EOSTokenID > 0 {
		stopIDs[cfgData.EOSTokenID] = struct{}{}
	}
	if cfgData.TextConfig.EOSTokenID > 0 {
		stopIDs[cfgData.TextConfig.EOSTokenID] = struct{}{}
	}

	return &goEngine{
		modelDir:           modelDir,
		tokenizer:          tok,
		imageTokenID:       cfgData.ImageTokenID,
		visionStartTokenID: cfgData.VisionStartTokenID,
		visionEndTokenID:   cfgData.VisionEndTokenID,
		stopTokenIDs:       stopIDs,
		embedSession:       embedSession,
		decoderSession:     decoderSession,
		visionSession:      visionSession,
	}, "", nil
}

func loadModelConfig(configPath string) (*modelConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read model config: %w", err)
	}
	var cfg modelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse model config: %w", err)
	}
	return &cfg, nil
}

func (e *goEngine) generate(ctx context.Context, prompt string, maxTokens int, images []ImageInput) (string, error) {
	if e == nil || e.tokenizer == nil {
		return "", fmt.Errorf("engine not initialized")
	}
	promptIDs := e.tokenizer.encode(prompt)
	if len(promptIDs) == 0 {
		return "", fmt.Errorf("prompt tokenization produced no tokens")
	}

	var imageFeatures []float32
	var imageFeatureRows int64
	if len(images) > 0 {
		feat, rows, err := e.extractImageFeatures(images[0])
		if err != nil {
			return "", err
		}
		imageFeatures = feat
		imageFeatureRows = rows
	}

	ids := make([]int64, 0, len(promptIDs)+3)
	if imageFeatureRows > 0 && e.visionStartTokenID > 0 && e.imageTokenID > 0 && e.visionEndTokenID > 0 {
		ids = append(ids, e.visionStartTokenID, e.imageTokenID, e.visionEndTokenID)
	}
	ids = append(ids, promptIDs...)
	if len(ids) > goMaxPromptTokens {
		ids = ids[len(ids)-goMaxPromptTokens:]
	}

	embedTensor, err := e.runEmbed(ids)
	if err != nil {
		return "", err
	}
	defer embedTensor.Destroy()

	if imageFeatureRows > 0 && len(imageFeatures) >= 1024 {
		injectImageFeatures(embedTensor.GetData(), ids, e.imageTokenID, imageFeatures, int(imageFeatureRows))
	}

	state, err := newInitialDecoderState()
	if err != nil {
		return "", err
	}
	defer func() { destroyValues(state) }()

	attentionMask, err := onesInt64Tensor(1, int64(len(ids)))
	if err != nil {
		return "", err
	}
	defer attentionMask.Destroy()

	positionIDs, err := makePositionIDs(int64(len(ids)), true)
	if err != nil {
		return "", err
	}
	defer positionIDs.Destroy()

	nextID, newState, err := e.runDecoderStep(embedTensor, attentionMask, positionIDs, state)
	if err != nil {
		return "", err
	}
	destroyValues(state)
	state = newState

	generated := make([]int64, 0, maxTokens)
	totalLen := int64(len(ids))
	for i := 0; i < maxTokens; i++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if e.isStopToken(nextID) {
			break
		}
		generated = append(generated, nextID)
		totalLen++

		stepEmbed, stepErr := e.runEmbed([]int64{nextID})
		if stepErr != nil {
			return "", stepErr
		}

		stepMask, stepErr := onesInt64Tensor(1, totalLen)
		if stepErr != nil {
			stepEmbed.Destroy()
			return "", stepErr
		}

		stepPos, stepErr := makeStepPositionIDs(totalLen - 1)
		if stepErr != nil {
			stepMask.Destroy()
			stepEmbed.Destroy()
			return "", stepErr
		}

		next, nextState, stepErr := e.runDecoderStep(stepEmbed, stepMask, stepPos, state)
		stepPos.Destroy()
		stepMask.Destroy()
		stepEmbed.Destroy()
		if stepErr != nil {
			return "", stepErr
		}
		destroyValues(state)
		state = nextState
		nextID = next
	}

	if len(generated) == 0 {
		return "", nil
	}
	return strings.TrimSpace(e.tokenizer.decode(generated)), nil
}

func (e *goEngine) runEmbed(ids []int64) (*ort.Tensor[float32], error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("empty token ids")
	}
	idsTensor, err := ort.NewTensor(ort.NewShape(1, int64(len(ids))), ids)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer idsTensor.Destroy()

	outputs := []ort.Value{nil}
	if err := e.embedSession.Run([]ort.Value{idsTensor}, outputs); err != nil {
		return nil, fmt.Errorf("embed inference failed: %w", err)
	}
	t, ok := outputs[0].(*ort.Tensor[float32])
	if !ok || t == nil {
		if outputs[0] != nil {
			_ = outputs[0].Destroy()
		}
		return nil, fmt.Errorf("embed output tensor type mismatch")
	}
	return t, nil
}

func (e *goEngine) runDecoderStep(
	inputsEmbeds *ort.Tensor[float32],
	attentionMask *ort.Tensor[int64],
	positionIDs *ort.Tensor[int64],
	state []ort.Value,
) (int64, []ort.Value, error) {
	if inputsEmbeds == nil || attentionMask == nil || positionIDs == nil {
		return 0, nil, fmt.Errorf("decoder step input tensors are nil")
	}
	decoderInputs := make([]ort.Value, 0, 3+len(state))
	decoderInputs = append(decoderInputs, inputsEmbeds, attentionMask, positionIDs)
	decoderInputs = append(decoderInputs, state...)

	outputs := make([]ort.Value, len(decoderOutputNames()))
	if err := e.decoderSession.Run(decoderInputs, outputs); err != nil {
		destroyValues(outputs)
		return 0, nil, fmt.Errorf("decoder inference failed: %w", err)
	}

	logitsTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok || logitsTensor == nil {
		destroyValues(outputs)
		return 0, nil, fmt.Errorf("decoder logits tensor type mismatch")
	}
	nextID, err := argmaxLastToken(logitsTensor)
	if err != nil {
		destroyValues(outputs)
		return 0, nil, err
	}
	_ = outputs[0].Destroy()

	nextState := make([]ort.Value, len(outputs)-1)
	copy(nextState, outputs[1:])
	return nextID, nextState, nil
}

func (e *goEngine) isStopToken(id int64) bool {
	_, ok := e.stopTokenIDs[id]
	return ok
}

func (e *goEngine) extractImageFeatures(img ImageInput) ([]float32, int64, error) {
	if e.visionSession == nil {
		return nil, 0, fmt.Errorf("vision session is unavailable")
	}
	raw, err := decodeBase64Payload(img.Data)
	if err != nil {
		return nil, 0, fmt.Errorf("decode image payload: %w", err)
	}
	pixelData, gridData, err := preprocessImageToPixelValues(raw)
	if err != nil {
		return nil, 0, err
	}

	pixelTensor, err := ort.NewTensor(ort.NewShape(int64(len(pixelData)/1536), 1536), pixelData)
	if err != nil {
		return nil, 0, fmt.Errorf("create pixel_values tensor: %w", err)
	}
	defer pixelTensor.Destroy()

	gridTensor, err := ort.NewTensor(ort.NewShape(1, 3), gridData)
	if err != nil {
		return nil, 0, fmt.Errorf("create image_grid_thw tensor: %w", err)
	}
	defer gridTensor.Destroy()

	outputs := []ort.Value{nil}
	if err := e.visionSession.Run([]ort.Value{pixelTensor, gridTensor}, outputs); err != nil {
		destroyValues(outputs)
		return nil, 0, fmt.Errorf("vision inference failed: %w", err)
	}
	featTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok || featTensor == nil {
		destroyValues(outputs)
		return nil, 0, fmt.Errorf("vision output tensor type mismatch")
	}
	data := append([]float32(nil), featTensor.GetData()...)
	shape := featTensor.GetShape()
	_ = outputs[0].Destroy()

	rows := int64(0)
	if len(shape) >= 1 {
		rows = shape[0]
	}
	if rows <= 0 {
		if len(data)%1024 != 0 {
			return nil, 0, fmt.Errorf("invalid vision feature size: %d", len(data))
		}
		rows = int64(len(data) / 1024)
	}
	return data, rows, nil
}

func preprocessImageToPixelValues(raw []byte) ([]float32, []int64, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("decode image bytes: %w", err)
	}
	rgba := resizeNearest(img, 32, 32)

	out := make([]float32, 4*1536) // t=1, h=2, w=2 => 4 patches.
	patchIdx := 0
	for ph := 0; ph < 2; ph++ {
		for pw := 0; pw < 2; pw++ {
			base := patchIdx * 1536
			idx := base
			for c := 0; c < 3; c++ {
				for t := 0; t < 2; t++ { // still image: duplicate 2 temporal slices.
					for y := 0; y < 16; y++ {
						for x := 0; x < 16; x++ {
							px := rgba.RGBAAt(pw*16+x, ph*16+y)
							var v float32
							switch c {
							case 0:
								v = float32(px.R) / 255.0
							case 1:
								v = float32(px.G) / 255.0
							default:
								v = float32(px.B) / 255.0
							}
							out[idx] = (v - 0.5) / 0.5
							idx++
						}
					}
				}
			}
			patchIdx++
		}
	}
	return out, []int64{1, 2, 2}, nil
}

func resizeNearest(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw := sb.Dx()
	sh := sb.Dy()
	if sw <= 0 || sh <= 0 {
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
		return dst
	}
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + (y*sh)/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + (x*sw)/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func decodeBase64Payload(s string) ([]byte, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil, fmt.Errorf("empty base64 payload")
	}
	if i := strings.Index(raw, ","); i >= 0 {
		prefix := strings.ToLower(raw[:i])
		if strings.Contains(prefix, "base64") {
			raw = raw[i+1:]
		}
	}
	if out, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return out, nil
	}
	return base64.RawStdEncoding.DecodeString(raw)
}

func decoderInputNames() []string {
	names := []string{"inputs_embeds", "attention_mask", "position_ids"}
	for i := 0; i < 24; i++ {
		if i%4 == 3 {
			names = append(names, fmt.Sprintf("past_key_values.%d.key", i), fmt.Sprintf("past_key_values.%d.value", i))
			continue
		}
		names = append(names, fmt.Sprintf("past_conv.%d", i), fmt.Sprintf("past_recurrent.%d", i))
	}
	return names
}

func decoderOutputNames() []string {
	names := []string{"logits"}
	for i := 0; i < 24; i++ {
		if i%4 == 3 {
			names = append(names, fmt.Sprintf("present.%d.key", i), fmt.Sprintf("present.%d.value", i))
			continue
		}
		names = append(names, fmt.Sprintf("present_conv.%d", i), fmt.Sprintf("present_recurrent.%d", i))
	}
	return names
}

func newInitialDecoderState() ([]ort.Value, error) {
	state := make([]ort.Value, 0, 48)
	for i := 0; i < 24; i++ {
		if i%4 == 3 {
			key, err := ort.NewTensor(ort.NewShape(1, 2, 0, 256), []float32{})
			if err != nil {
				destroyValues(state)
				return nil, fmt.Errorf("create initial kv key tensor: %w", err)
			}
			val, err := ort.NewTensor(ort.NewShape(1, 2, 0, 256), []float32{})
			if err != nil {
				key.Destroy()
				destroyValues(state)
				return nil, fmt.Errorf("create initial kv value tensor: %w", err)
			}
			state = append(state, key, val)
			continue
		}
		conv, err := ort.NewTensor(ort.NewShape(1, 6144, 4), make([]float32, 1*6144*4))
		if err != nil {
			destroyValues(state)
			return nil, fmt.Errorf("create initial conv tensor: %w", err)
		}
		recurrent, err := ort.NewTensor(ort.NewShape(1, 16, 128, 128), make([]float32, 1*16*128*128))
		if err != nil {
			conv.Destroy()
			destroyValues(state)
			return nil, fmt.Errorf("create initial recurrent tensor: %w", err)
		}
		state = append(state, conv, recurrent)
	}
	return state, nil
}

func onesInt64Tensor(rows, cols int64) (*ort.Tensor[int64], error) {
	if rows <= 0 || cols <= 0 {
		return nil, fmt.Errorf("invalid mask shape: [%d,%d]", rows, cols)
	}
	data := make([]int64, rows*cols)
	for i := range data {
		data[i] = 1
	}
	return ort.NewTensor(ort.NewShape(rows, cols), data)
}

func makePositionIDs(seqLen int64, firstPass bool) (*ort.Tensor[int64], error) {
	if seqLen <= 0 {
		return nil, fmt.Errorf("invalid seq len: %d", seqLen)
	}
	data := make([]int64, 3*seqLen)
	for i := int64(0); i < seqLen; i++ {
		v := i
		if !firstPass {
			v = seqLen - 1
		}
		data[i] = v
		data[seqLen+i] = v
		data[2*seqLen+i] = v
	}
	return ort.NewTensor(ort.NewShape(3, 1, seqLen), data)
}

func makeStepPositionIDs(pos int64) (*ort.Tensor[int64], error) {
	data := []int64{pos, pos, pos}
	return ort.NewTensor(ort.NewShape(3, 1, 1), data)
}

func injectImageFeatures(embedData []float32, tokenIDs []int64, imageTokenID int64, features []float32, rows int) {
	if len(embedData) == 0 || len(tokenIDs) == 0 || imageTokenID <= 0 || len(features) < 1024 || rows <= 0 {
		return
	}
	seqLen := len(tokenIDs)
	if len(embedData) < seqLen*1024 {
		return
	}
	row := 0
	for i, id := range tokenIDs {
		if id != imageTokenID {
			continue
		}
		if row >= rows {
			break
		}
		dstStart := i * 1024
		srcStart := row * 1024
		copy(embedData[dstStart:dstStart+1024], features[srcStart:srcStart+1024])
		row++
	}
}

func argmaxLastToken(logits *ort.Tensor[float32]) (int64, error) {
	if logits == nil {
		return 0, fmt.Errorf("nil logits tensor")
	}
	shape := logits.GetShape()
	if len(shape) < 3 {
		return 0, fmt.Errorf("invalid logits shape: %v", shape)
	}
	seqLen := int(shape[1])
	vocab := int(shape[2])
	if seqLen <= 0 || vocab <= 0 {
		return 0, fmt.Errorf("invalid logits dims: seq=%d vocab=%d", seqLen, vocab)
	}
	data := logits.GetData()
	if len(data) < seqLen*vocab {
		return 0, fmt.Errorf("logits data too short: got=%d want>=%d", len(data), seqLen*vocab)
	}
	start := (seqLen - 1) * vocab
	bestIdx := 0
	bestVal := data[start]
	for i := 1; i < vocab; i++ {
		v := data[start+i]
		if v > bestVal {
			bestVal = v
			bestIdx = i
		}
	}
	return int64(bestIdx), nil
}

func destroyValues(values []ort.Value) {
	for _, v := range values {
		if v != nil {
			_ = v.Destroy()
		}
	}
}
