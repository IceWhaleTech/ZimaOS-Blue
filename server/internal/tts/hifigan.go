//go:build espeak

package tts

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/hifi-gan/src
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../third_party/hifi-gan/build -lhifigan
#cgo linux LDFLAGS: -L${SRCDIR}/../../../third_party/hifi-gan/build -lhifigan -lpthread

#include <stdlib.h>
#include <stdint.h>

// HiFi-GAN vocoder interface
typedef struct {
    void* model;
    int sample_rate;
} hifigan_t;

// Initialize vocoder with model path
hifigan_t* hifigan_init(const char* model_path, int sample_rate);

// Process PCM audio (input_samples -> output_samples)
// Returns number of output samples, or -1 on error
int hifigan_process(hifigan_t* vocoder, const int16_t* input, int input_len, int16_t* output, int output_capacity);

// Cleanup
void hifigan_cleanup(hifigan_t* vocoder);
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// VocoderInstance wraps HiFi-GAN vocoder
type VocoderInstance struct {
	model      *C.hifigan_t
	sampleRate int
	mu         sync.Mutex
}

// InitVocoder initializes HiFi-GAN vocoder (lazy load)
func InitVocoder(modelPath string, sampleRate int) (*VocoderInstance, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	model := C.hifigan_init(cPath, C.int(sampleRate))
	if model == nil {
		return nil, fmt.Errorf("failed to initialize HiFi-GAN vocoder")
	}

	return &VocoderInstance{
		model:      model,
		sampleRate: sampleRate,
	}, nil
}

// Process applies vocoder to PCM audio
func (v *VocoderInstance) Process(input []int16) ([]int16, error) {
	if v == nil || v.model == nil {
		return input, nil // passthrough if vocoder not available
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Allocate output buffer (vocoder may expand audio)
	outputCapacity := len(input) * 2
	output := make([]int16, outputCapacity)

	inputPtr := (*C.int16_t)(unsafe.Pointer(&input[0]))
	outputPtr := (*C.int16_t)(unsafe.Pointer(&output[0]))

	numSamples := C.hifigan_process(v.model, inputPtr, C.int(len(input)), outputPtr, C.int(outputCapacity))
	if numSamples < 0 {
		return nil, fmt.Errorf("vocoder processing failed")
	}

	return output[:numSamples], nil
}

// Close releases vocoder resources
func (v *VocoderInstance) Close() {
	if v != nil && v.model != nil {
		C.hifigan_cleanup(v.model)
		v.model = nil
	}
}
