package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
)

const defaultTokenID = int64(248044) // eos_token_id from config.json

func defaultModelDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".zimaos-blue", "data", "models", "qwen3.5-0.8b-onnx-q4")
	}
	return "data/models/qwen3.5-0.8b-onnx-q4"
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

func argmax(values []float32) (int, float32) {
	if len(values) == 0 {
		return -1, 0
	}
	idx := 0
	best := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > best {
			best = values[i]
			idx = i
		}
	}
	return idx, best
}

func requireFiles(modelDir string) error {
	needed := []string{
		"config.json",
		"tokenizer.json",
		"onnx/embed_tokens_q4.onnx",
		"onnx/embed_tokens_q4.onnx_data",
		"onnx/decoder_model_merged_q4.onnx",
		"onnx/decoder_model_merged_q4.onnx_data",
		"onnx/vision_encoder_q4.onnx",
		"onnx/vision_encoder_q4.onnx_data",
	}
	for _, rel := range needed {
		p := filepath.Join(modelDir, rel)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("missing required file: %s (%w)", p, err)
		}
	}
	return nil
}

func ensureRuntime(dataDir string) error {
	onnx.SetDataDir(dataDir)
	if libPath := onnx.RuntimeLibPath(dataDir); libPath != "" {
		onnx.SetLibraryPath(libPath)
		return nil
	}
	libPath, err := onnx.EnsureRuntime(dataDir)
	if err != nil {
		return fmt.Errorf("ensure onnxruntime library: %w", err)
	}
	onnx.SetLibraryPath(libPath)
	return nil
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
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

// Qwen3.5 vision encoder expects:
// - pixel_values: [num_patches, 1536] = [num_patches, 3*2*16*16]
// - image_grid_thw: [num_images, 3] where each row is [t, h, w].
func buildVisionInputsFromImage(path string) ([]float32, []int64, error) {
	img, err := decodeImage(path)
	if err != nil {
		return nil, nil, err
	}
	rgba := resizeNearest(img, 32, 32)

	out := make([]float32, 4*1536) // t=1, h=2, w=2 => 4 patches
	patchIdx := 0
	for ph := 0; ph < 2; ph++ {
		for pw := 0; pw < 2; pw++ {
			base := patchIdx * 1536
			idx := base
			for c := 0; c < 3; c++ {
				for t := 0; t < 2; t++ { // still image: duplicate frame
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
							// Match config: rescale + normalize (mean=0.5,std=0.5)
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

func buildVisionInputsSynthetic() ([]float32, []int64) {
	return make([]float32, 4*1536), []int64{1, 2, 2}
}

func main() {
	modelDir := flag.String("model-dir", defaultModelDir(), "Qwen3.5 ONNX model directory")
	dataDir := flag.String("data-dir", "", "Data directory that contains onnxruntime/ (default: parent of parent of model-dir)")
	tokenID := flag.Int64("token-id", defaultTokenID, "Token ID used for one-step text smoke test")
	imagePath := flag.String("image", "", "Optional image path for vision smoke test")
	skipVision := flag.Bool("skip-vision", false, "Skip vision encoder smoke test")
	flag.Parse()

	if *modelDir == "" {
		fmt.Fprintln(os.Stderr, "model-dir is required")
		os.Exit(2)
	}
	if err := requireFiles(*modelDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	effectiveDataDir := *dataDir
	if effectiveDataDir == "" {
		effectiveDataDir = filepath.Dir(filepath.Dir(*modelDir))
	}
	if err := ensureRuntime(effectiveDataDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	embedPath := filepath.Join(*modelDir, "onnx", "embed_tokens_q4.onnx")
	decoderPath := filepath.Join(*modelDir, "onnx", "decoder_model_merged_q4.onnx")
	visionPath := filepath.Join(*modelDir, "onnx", "vision_encoder_q4.onnx")

	embedSession, err := onnx.NewDynamicSession(embedPath, []string{"input_ids"}, []string{"inputs_embeds"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create embed session: %v\n", err)
		os.Exit(1)
	}
	defer embedSession.Close()

	idsTensor, err := ort.NewTensor(ort.NewShape(1, 1), []int64{*tokenID})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create input_ids tensor: %v\n", err)
		os.Exit(1)
	}
	defer idsTensor.Destroy()

	embedOut := []ort.Value{nil}
	if err := embedSession.Run([]ort.Value{idsTensor}, embedOut); err != nil {
		fmt.Fprintf(os.Stderr, "embed run failed: %v\n", err)
		os.Exit(1)
	}
	if embedOut[0] == nil {
		fmt.Fprintln(os.Stderr, "embed output is nil")
		os.Exit(1)
	}
	embedTensor, ok := embedOut[0].(*ort.Tensor[float32])
	if !ok {
		fmt.Fprintf(os.Stderr, "embed output type mismatch: %T\n", embedOut[0])
		os.Exit(1)
	}
	embedData := append([]float32(nil), embedTensor.GetData()...)
	_ = embedOut[0].Destroy()

	inputsEmbeds, err := ort.NewTensor(ort.NewShape(1, 1, 1024), embedData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create inputs_embeds tensor: %v\n", err)
		os.Exit(1)
	}
	defer inputsEmbeds.Destroy()

	attentionMask, err := ort.NewTensor(ort.NewShape(1, 1), []int64{1})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create attention_mask tensor: %v\n", err)
		os.Exit(1)
	}
	defer attentionMask.Destroy()

	positionIDs, err := ort.NewTensor(ort.NewShape(3, 1, 1), []int64{0, 0, 0})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create position_ids tensor: %v\n", err)
		os.Exit(1)
	}
	defer positionIDs.Destroy()

	convTensor, err := ort.NewTensor(ort.NewShape(1, 6144, 4), make([]float32, 1*6144*4))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create past_conv tensor: %v\n", err)
		os.Exit(1)
	}
	defer convTensor.Destroy()

	recurrentTensor, err := ort.NewTensor(ort.NewShape(1, 16, 128, 128), make([]float32, 1*16*128*128))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create past_recurrent tensor: %v\n", err)
		os.Exit(1)
	}
	defer recurrentTensor.Destroy()

	kvTensor, err := ort.NewTensor(ort.NewShape(1, 2, 0, 256), []float32{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create past_key_values tensor: %v\n", err)
		os.Exit(1)
	}
	defer kvTensor.Destroy()

	decoderInputs := []ort.Value{inputsEmbeds, attentionMask, positionIDs}
	for i := 0; i < 24; i++ {
		if i%4 == 3 {
			decoderInputs = append(decoderInputs, kvTensor, kvTensor)
			continue
		}
		decoderInputs = append(decoderInputs, convTensor, recurrentTensor)
	}

	decoderSession, err := onnx.NewDynamicSession(decoderPath, decoderInputNames(), []string{"logits"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create decoder session: %v\n", err)
		os.Exit(1)
	}
	defer decoderSession.Close()

	decoderOut := []ort.Value{nil}
	if err := decoderSession.Run(decoderInputs, decoderOut); err != nil {
		fmt.Fprintf(os.Stderr, "decoder run failed: %v\n", err)
		os.Exit(1)
	}
	if decoderOut[0] == nil {
		fmt.Fprintln(os.Stderr, "decoder output is nil")
		os.Exit(1)
	}
	logitsTensor, ok := decoderOut[0].(*ort.Tensor[float32])
	if !ok {
		fmt.Fprintf(os.Stderr, "decoder output type mismatch: %T\n", decoderOut[0])
		os.Exit(1)
	}
	topID, topLogit := argmax(logitsTensor.GetData())
	fmt.Printf("text_ok logits_shape=%s vocab=%d top_token_id=%d top_logit=%.4f\n",
		logitsTensor.GetShape().String(), len(logitsTensor.GetData()), topID, topLogit)
	_ = decoderOut[0].Destroy()

	if *skipVision {
		fmt.Println("vision_skip=true")
		fmt.Println("verify_result=ok")
		return
	}

	visionSession, err := onnx.NewDynamicSession(visionPath, []string{"pixel_values", "image_grid_thw"}, []string{"image_features"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create vision session: %v\n", err)
		os.Exit(1)
	}
	defer visionSession.Close()

	var (
		pixelData []float32
		gridData  []int64
	)
	if *imagePath != "" {
		pixelData, gridData, err = buildVisionInputsFromImage(*imagePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build vision input from image failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		pixelData, gridData = buildVisionInputsSynthetic()
	}

	if len(pixelData) == 0 || len(gridData) != 3 {
		fmt.Fprintln(os.Stderr, "invalid vision input data")
		os.Exit(1)
	}
	if len(pixelData)%1536 != 0 {
		fmt.Fprintln(os.Stderr, "pixel_data length must be multiple of 1536")
		os.Exit(1)
	}
	numPatches := int64(len(pixelData) / 1536)

	pixelTensor, err := ort.NewTensor(ort.NewShape(numPatches, 1536), pixelData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create pixel_values tensor: %v\n", err)
		os.Exit(1)
	}
	defer pixelTensor.Destroy()

	gridTensor, err := ort.NewTensor(ort.NewShape(1, 3), gridData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create image_grid_thw tensor: %v\n", err)
		os.Exit(1)
	}
	defer gridTensor.Destroy()

	visionOut := []ort.Value{nil}
	if err := visionSession.Run([]ort.Value{pixelTensor, gridTensor}, visionOut); err != nil {
		fmt.Fprintf(os.Stderr, "vision run failed: %v\n", err)
		os.Exit(1)
	}
	if visionOut[0] == nil {
		fmt.Fprintln(os.Stderr, "vision output is nil")
		os.Exit(1)
	}
	featuresTensor, ok := visionOut[0].(*ort.Tensor[float32])
	if !ok {
		fmt.Fprintf(os.Stderr, "vision output type mismatch: %T\n", visionOut[0])
		os.Exit(1)
	}
	fmt.Printf("vision_ok features_shape=%s features_size=%d image_input=%t\n",
		featuresTensor.GetShape().String(), len(featuresTensor.GetData()), *imagePath != "")
	_ = visionOut[0].Destroy()
	fmt.Println("verify_result=ok")
}
