//go:build cgo

package scenecompose

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
)

func (m *U2NetPModelManager) inferMask(_ context.Context, src image.Image) (*CutoutResult, error) {
	if src == nil {
		return nil, fmt.Errorf("empty image")
	}
	if err := m.ensureSessionLocked(); err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	input, originalW, originalH := prepareModelInput(src, m.inputWidth, m.inputHeight)
	inputTensor, err := ort.NewTensor(ort.NewShape(1, 3, int64(m.inputHeight), int64(m.inputWidth)), input)
	if err != nil {
		return nil, fmt.Errorf("create u2netp input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputs := []ort.Value{nil}
	if err := m.session.Run([]ort.Value{inputTensor}, outputs); err != nil {
		return nil, fmt.Errorf("run u2netp: %w", err)
	}
	if outputs[0] == nil {
		return nil, fmt.Errorf("u2netp returned empty output")
	}
	defer outputs[0].Destroy()

	maskData, outW, outH, err := extractMaskData(outputs[0])
	if err != nil {
		return nil, err
	}
	mask := resizeMask(maskData, outW, outH, originalW, originalH, bounds.Min.X, bounds.Min.Y)
	mask = featherMask(mask, 3)
	return &CutoutResult{
		Image: applyMask(src, mask),
		Mask:  mask,
	}, nil
}

func (m *U2NetPModelManager) ensureSessionLocked() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		return nil
	}
	onnx.SetDataDir(m.dataDir)
	if libPath := onnx.RuntimeLibPath(m.dataDir); libPath != "" {
		onnx.SetLibraryPath(libPath)
	}
	inputs, outputs, err := ort.GetInputOutputInfo(m.modelPath())
	if err != nil {
		return fmt.Errorf("inspect u2netp model io: %w", err)
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return fmt.Errorf("u2netp model has no IO metadata")
	}
	inputName := strings.TrimSpace(inputs[0].Name)
	outputName := strings.TrimSpace(outputs[0].Name)
	if inputName == "" || outputName == "" {
		return fmt.Errorf("u2netp model io names are empty")
	}
	inputH, inputW := 320, 320
	if len(inputs[0].Dimensions) >= 4 {
		if inputs[0].Dimensions[2] > 0 {
			inputH = int(inputs[0].Dimensions[2])
		}
		if inputs[0].Dimensions[3] > 0 {
			inputW = int(inputs[0].Dimensions[3])
		}
	}
	session, err := onnx.NewDynamicSession(m.modelPath(), []string{inputName}, []string{outputName})
	if err != nil {
		return fmt.Errorf("load u2netp session: %w", err)
	}
	m.session = session
	m.inputName = inputName
	m.outputName = outputName
	m.inputWidth = inputW
	m.inputHeight = inputH
	return nil
}

func prepareModelInput(src image.Image, width, height int) ([]float32, int, int) {
	resized := resizeTo(src, width, height)
	data := make([]float32, 3*width*height)
	originalW := src.Bounds().Dx()
	originalH := src.Bounds().Dy()
	index := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBAModel.Convert(resized.At(x, y)).(color.RGBA)
			r := ((float32(c.R) / 255.0) - 0.485) / 0.229
			g := ((float32(c.G) / 255.0) - 0.456) / 0.224
			b := ((float32(c.B) / 255.0) - 0.406) / 0.225
			data[index] = r
			data[index+width*height] = g
			data[index+2*width*height] = b
			index++
		}
	}
	return data, originalW, originalH
}

func extractMaskData(value ort.Value) ([]float32, int, int, error) {
	tensor, ok := value.(*ort.Tensor[float32])
	if !ok {
		return nil, 0, 0, fmt.Errorf("u2netp output tensor type %T is not float32", value)
	}
	shape := tensor.GetShape()
	width := 0
	height := 0
	switch len(shape) {
	case 4:
		height = int(shape[2])
		width = int(shape[3])
	case 3:
		height = int(shape[1])
		width = int(shape[2])
	default:
		return nil, 0, 0, fmt.Errorf("unexpected u2netp output shape %v", shape)
	}
	if width <= 0 || height <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid u2netp output shape %v", shape)
	}
	raw := append([]float32(nil), tensor.GetData()...)
	minValue := float32(math.MaxFloat32)
	maxValue := float32(-math.MaxFloat32)
	for _, value := range raw {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	scale := maxValue - minValue
	if scale <= 1e-6 {
		scale = 1
	}
	for i, value := range raw {
		raw[i] = (value - minValue) / scale
	}
	return raw, width, height, nil
}

func resizeMask(data []float32, width, height, targetW, targetH, offsetX, offsetY int) *image.Alpha {
	mask := image.NewAlpha(image.Rect(offsetX, offsetY, offsetX+targetW, offsetY+targetH))
	for y := 0; y < targetH; y++ {
		srcY := int(float64(y) / float64(maxInt(1, targetH-1)) * float64(maxInt(1, height-1)))
		for x := 0; x < targetW; x++ {
			srcX := int(float64(x) / float64(maxInt(1, targetW-1)) * float64(maxInt(1, width-1)))
			value := data[srcY*width+srcX]
			alpha := clampUint8(float64(value * 255))
			mask.SetAlpha(offsetX+x, offsetY+y, color.Alpha{A: alpha})
		}
	}
	return mask
}
