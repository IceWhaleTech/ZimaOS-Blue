package tts

import (
	"math"
)

// AudioPreprocessor handles audio preprocessing before vocoder
type AudioPreprocessor struct {
	sampleRate int
	nFFT       int
	hopLength  int
	nMels      int
	melFmin    float64
	melFmax    float64
}

// NewAudioPreprocessor creates a new audio preprocessor
func NewAudioPreprocessor(sampleRate int) *AudioPreprocessor {
	return &AudioPreprocessor{
		sampleRate: sampleRate,
		nFFT:       1024,
		hopLength:  256,
		nMels:      80,
		melFmin:    0,
		melFmax:    8000,
	}
}

// ApplyEQ applies EQ filter (boost 1kHz by 3dB)
func (p *AudioPreprocessor) ApplyEQ(samples []int16) []int16 {
	// Simple peaking EQ at 1kHz with Q=1, gain=3dB
	// Using biquad filter coefficients
	const (
		centerFreq = 1000.0
		Q          = 1.0
		gainDB     = 3.0
	)

	w0 := 2 * math.Pi * centerFreq / float64(p.sampleRate)
	alpha := math.Sin(w0) / (2 * Q)
	A := math.Pow(10, gainDB/40)

	b0 := 1 + alpha*A
	b1 := -2 * math.Cos(w0)
	b2 := 1 - alpha*A
	a0 := 1 + alpha/A
	a1 := -2 * math.Cos(w0)
	a2 := 1 - alpha/A

	// Normalize
	b0 /= a0
	b1 /= a0
	b2 /= a0
	a1 /= a0
	a2 /= a0

	// Apply biquad filter
	result := make([]int16, len(samples))
	var x1, x2, y1, y2 float64

	for i, sample := range samples {
		x0 := float64(sample)
		y0 := b0*x0 + b1*x1 + b2*x2 - a1*y1 - a2*y2

		// Clamp to int16 range
		if y0 > 32767 {
			y0 = 32767
		} else if y0 < -32768 {
			y0 = -32768
		}

		result[i] = int16(y0)
		x2, x1 = x1, x0
		y2, y1 = y1, y0
	}

	return result
}

// ApplyLowpass applies lowpass filter at 7kHz
func (p *AudioPreprocessor) ApplyLowpass(samples []int16) []int16 {
	const cutoffFreq = 7000.0

	w0 := 2 * math.Pi * cutoffFreq / float64(p.sampleRate)
	alpha := math.Sin(w0) / 2

	b0 := (1 - math.Cos(w0)) / 2
	b1 := 1 - math.Cos(w0)
	b2 := (1 - math.Cos(w0)) / 2
	a0 := 1 + alpha
	a1 := -2 * math.Cos(w0)
	a2 := 1 - alpha

	// Normalize
	b0 /= a0
	b1 /= a0
	b2 /= a0
	a1 /= a0
	a2 /= a0

	// Apply biquad filter
	result := make([]int16, len(samples))
	var x1, x2, y1, y2 float64

	for i, sample := range samples {
		x0 := float64(sample)
		y0 := b0*x0 + b1*x1 + b2*x2 - a1*y1 - a2*y2

		// Clamp to int16 range
		if y0 > 32767 {
			y0 = 32767
		} else if y0 < -32768 {
			y0 = -32768
		}

		result[i] = int16(y0)
		x2, x1 = x1, x0
		y2, y1 = y1, y0
	}

	return result
}

// Preprocess applies full preprocessing chain
func (p *AudioPreprocessor) Preprocess(samples []int16) []int16 {
	// Chain: EQ -> Lowpass
	samples = p.ApplyEQ(samples)
	samples = p.ApplyLowpass(samples)
	return samples
}
