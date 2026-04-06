//go:build espeak && (!linux || !cgo)

package tts

import "fmt"

// VocoderInstance is a compatibility stub when HiFi-GAN is unavailable.
type VocoderInstance struct{}

// InitVocoder reports that HiFi-GAN requires linux+cgo ONNX runtime.
func InitVocoder(string, int) (*VocoderInstance, error) {
	return nil, fmt.Errorf("HiFi-GAN vocoder requires linux with cgo-enabled onnx runtime")
}

// Process falls back to passthrough audio when the vocoder is unavailable.
func (v *VocoderInstance) Process(input []int16) ([]int16, error) {
	_ = v
	return input, nil
}

// Close is a no-op for the compatibility stub.
func (v *VocoderInstance) Close() {
	_ = v
}
