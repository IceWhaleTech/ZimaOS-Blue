//go:build espeak && linux && cgo

package tts

import (
	"fmt"
	"math"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
)

// VocoderInstance wraps HiFi-GAN ONNX vocoder.
// Input: mel spectrogram [1, 80, T], Output: waveform [1, 1, T*256].
type VocoderInstance struct {
	session *onnx.DynamicSession
	melCfg  MelConfig
	mu      sync.Mutex
}

// InitVocoder loads a HiFi-GAN ONNX model.
func InitVocoder(modelPath string, sampleRate int) (*VocoderInstance, error) {
	// Ensure ONNX Runtime is available
	dataDir := filepath.Dir(filepath.Dir(modelPath)) // vocoder/hifigan_v3.onnx → data dir
	onnx.SetDataDir(dataDir)
	if libPath := onnx.RuntimeLibPath(dataDir); libPath != "" {
		onnx.SetLibraryPath(libPath)
	} else if libPath, err := onnx.EnsureRuntime(dataDir); err == nil {
		onnx.SetLibraryPath(libPath)
	}

	session, err := onnx.NewDynamicSession(
		modelPath,
		[]string{"mel"},
		[]string{"audio"},
	)
	if err != nil {
		return nil, fmt.Errorf("load HiFi-GAN ONNX: %w", err)
	}

	return &VocoderInstance{
		session: session,
		melCfg:  DefaultMelConfig(), // 22050 Hz, matching eSpeak output
	}, nil
}

// Process converts PCM int16 → mel spectrogram → HiFi-GAN → PCM int16.
// Input and output are both at 22050 Hz (eSpeak native = HiFi-GAN V3 native).
func (v *VocoderInstance) Process(input []int16) ([]int16, error) {
	if v == nil || v.session == nil {
		return input, nil // passthrough if not available
	}

	if len(input) == 0 {
		return input, nil
	}

	// Compute mel spectrogram: [nMels][timeFrames]
	mel := ComputeMelSpectrogram(input, v.melCfg)
	if mel == nil || len(mel) == 0 || len(mel[0]) == 0 {
		return input, nil // fallback to passthrough
	}

	nMels := len(mel)
	nFrames := len(mel[0])

	// Flatten to [1, nMels, nFrames] for ONNX
	flatMel := make([]float32, nMels*nFrames)
	for m := 0; m < nMels; m++ {
		copy(flatMel[m*nFrames:], mel[m])
	}

	// Create input tensor
	melTensor, err := ort.NewTensor(ort.NewShape(1, int64(nMels), int64(nFrames)), flatMel)
	if err != nil {
		return nil, fmt.Errorf("create mel tensor: %w", err)
	}
	defer melTensor.Destroy()

	// Run inference (session not thread-safe)
	outputs := []ort.Value{nil} // auto-allocate output
	v.mu.Lock()
	err = v.session.Run([]ort.Value{melTensor}, outputs)
	v.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("HiFi-GAN inference: %w", err)
	}
	if outputs[0] != nil {
		defer outputs[0].Destroy()
	}

	if outputs[0] == nil {
		return input, nil
	}

	// Output is [1, 1, audioLength] float32 in [-1, 1]
	audioTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type")
	}
	audioData := audioTensor.GetData()

	// Convert float32 [-1,1] → int16
	result := make([]int16, len(audioData))
	for i, sample := range audioData {
		s := float64(sample)
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		result[i] = int16(math.Round(s * 32767.0))
	}

	return result, nil
}

// Close releases ONNX session resources.
func (v *VocoderInstance) Close() {
	if v != nil && v.session != nil {
		v.session.Close()
		v.session = nil
	}
}
