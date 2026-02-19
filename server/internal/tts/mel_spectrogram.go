package tts

import (
	"math"
	"math/cmplx"
)

// MelConfig holds mel spectrogram parameters matching HiFi-GAN V3 config.
type MelConfig struct {
	SampleRate int
	NFFT       int
	HopLength  int
	WinLength  int
	NMels      int
	FMin       float64
	FMax       float64
}

// DefaultMelConfig returns HiFi-GAN V3 default mel parameters.
func DefaultMelConfig() MelConfig {
	return MelConfig{
		SampleRate: 22050,
		NFFT:       1024,
		HopLength:  256,
		WinLength:  1024,
		NMels:      80,
		FMin:       0,
		FMax:       8000,
	}
}

// ComputeMelSpectrogram converts PCM int16 samples to a log-mel spectrogram.
// Returns shape [nMels][timeFrames] matching HiFi-GAN input format.
func ComputeMelSpectrogram(samples []int16, cfg MelConfig) [][]float32 {
	// Convert int16 to float64 normalized to [-1, 1]
	signal := make([]float64, len(samples))
	for i, s := range samples {
		signal[i] = float64(s) / 32768.0
	}

	// Pad signal to center frames (like librosa)
	padLen := cfg.NFFT / 2
	padded := make([]float64, padLen+len(signal)+padLen)
	// Reflect padding
	for i := 0; i < padLen; i++ {
		idx := padLen - i
		if idx >= len(signal) {
			idx = len(signal) - 1
		}
		padded[i] = signal[idx]
	}
	copy(padded[padLen:], signal)
	for i := 0; i < padLen; i++ {
		idx := len(signal) - 2 - i
		if idx < 0 {
			idx = 0
		}
		padded[padLen+len(signal)+i] = signal[idx]
	}

	// Hann window
	window := hannWindow(cfg.WinLength)

	// Number of frames
	nFrames := 1 + (len(padded)-cfg.NFFT)/cfg.HopLength
	if nFrames <= 0 {
		return nil
	}

	// STFT → power spectrogram
	nFreqs := cfg.NFFT/2 + 1
	powerSpec := make([][]float64, nFrames)
	for t := 0; t < nFrames; t++ {
		offset := t * cfg.HopLength
		frame := make([]complex128, cfg.NFFT)
		for i := 0; i < cfg.WinLength && offset+i < len(padded); i++ {
			frame[i] = complex(padded[offset+i]*window[i], 0)
		}
		fft(frame)
		powerSpec[t] = make([]float64, nFreqs)
		for i := 0; i < nFreqs; i++ {
			powerSpec[t][i] = real(frame[i])*real(frame[i]) + imag(frame[i])*imag(frame[i])
		}
	}

	// Mel filterbank
	melBank := melFilterbank(cfg.NMels, nFreqs, cfg.SampleRate, cfg.FMin, cfg.FMax)

	// Apply mel filterbank and log scale
	mel := make([][]float32, cfg.NMels)
	for m := 0; m < cfg.NMels; m++ {
		mel[m] = make([]float32, nFrames)
		for t := 0; t < nFrames; t++ {
			var sum float64
			for f := 0; f < nFreqs; f++ {
				sum += melBank[m][f] * powerSpec[t][f]
			}
			// Log scale with floor to avoid log(0)
			if sum < 1e-10 {
				sum = 1e-10
			}
			mel[m][t] = float32(math.Log(sum))
		}
	}

	return mel
}

// hannWindow generates a periodic Hann window of given length.
func hannWindow(length int) []float64 {
	w := make([]float64, length)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(length)))
	}
	return w
}

// melFilterbank creates a mel-scale filterbank matrix [nMels][nFreqs].
func melFilterbank(nMels, nFreqs, sampleRate int, fMin, fMax float64) [][]float64 {
	// Hz to mel conversion (HTK formula)
	hzToMel := func(hz float64) float64 {
		return 2595.0 * math.Log10(1.0+hz/700.0)
	}
	melToHz := func(mel float64) float64 {
		return 700.0 * (math.Pow(10.0, mel/2595.0) - 1.0)
	}

	melMin := hzToMel(fMin)
	melMax := hzToMel(fMax)

	// Equally spaced mel points
	melPoints := make([]float64, nMels+2)
	for i := range melPoints {
		melPoints[i] = melMin + float64(i)*(melMax-melMin)/float64(nMels+1)
	}

	// Convert to frequency bin indices
	fftFreqs := make([]float64, nMels+2)
	for i := range fftFreqs {
		fftFreqs[i] = math.Floor((float64(nFreqs*2-1)*melToHz(melPoints[i])/float64(sampleRate) + 1) / 1)
		// Actually: bin = floor((nfft+1) * hz / sr)
		fftFreqs[i] = math.Floor(float64(nFreqs*2-1) * melToHz(melPoints[i]) / float64(sampleRate))
	}

	bank := make([][]float64, nMels)
	for m := 0; m < nMels; m++ {
		bank[m] = make([]float64, nFreqs)
		left := fftFreqs[m]
		center := fftFreqs[m+1]
		right := fftFreqs[m+2]

		for f := 0; f < nFreqs; f++ {
			ff := float64(f)
			if ff > left && ff <= center && center > left {
				bank[m][f] = (ff - left) / (center - left)
			} else if ff > center && ff < right && right > center {
				bank[m][f] = (right - ff) / (right - center)
			}
		}
	}
	return bank
}

// fft performs in-place Cooley-Tukey FFT on a power-of-2 length slice.
func fft(a []complex128) {
	n := len(a)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}

	// Butterfly operations
	for length := 2; length <= n; length <<= 1 {
		angle := -2.0 * math.Pi / float64(length)
		wn := cmplx.Rect(1, angle)
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for k := 0; k < length/2; k++ {
				u := a[i+k]
				v := w * a[i+k+length/2]
				a[i+k] = u + v
				a[i+k+length/2] = u - v
				w *= wn
			}
		}
	}
}
