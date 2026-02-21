//go:build darwin

package tts

import (
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"testing"
)

// BenchmarkAiffcToWav_InProcess benchmarks the pure Go AIFF-C→WAV conversion.
func BenchmarkAiffcToWav_InProcess(b *testing.B) {
	aiffData := generateRealAIFF(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := aiffcToWav(aiffData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAiffcToWav_Afconvert benchmarks the old afconvert fork approach.
func BenchmarkAiffcToWav_Afconvert(b *testing.B) {
	aiffData := generateRealAIFF(b)

	tmpIn, err := os.CreateTemp("", "bench_aiff_*.aiff")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpIn.Name())
	tmpIn.Write(aiffData)
	tmpIn.Close()

	tmpOut := tmpIn.Name() + ".wav"
	defer os.Remove(tmpOut)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16", tmpIn.Name(), tmpOut)
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("afconvert failed: %v: %s", err, out)
		}
		_, err := os.ReadFile(tmpOut)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFullSynthesize_InProcess benchmarks say + in-process conversion.
func BenchmarkFullSynthesize_InProcess(b *testing.B) {
	if _, err := exec.LookPath("say"); err != nil {
		b.Skip("say not available")
	}

	tmpFile := b.TempDir() + "/bench.aiff"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("say", "-o", tmpFile, "Hello, this is a performance test.")
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("say failed: %v: %s", err, out)
		}
		aiffData, err := os.ReadFile(tmpFile)
		if err != nil {
			b.Fatal(err)
		}
		_, err = aiffcToWav(aiffData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFullSynthesize_Afconvert benchmarks say + afconvert (old approach).
func BenchmarkFullSynthesize_Afconvert(b *testing.B) {
	if _, err := exec.LookPath("say"); err != nil {
		b.Skip("say not available")
	}

	tmpFile := b.TempDir() + "/bench.aiff"
	wavFile := tmpFile + ".wav"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("say", "-o", tmpFile, "Hello, this is a performance test.")
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("say failed: %v: %s", err, out)
		}
		cmd = exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16", tmpFile, wavFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("afconvert failed: %v: %s", err, out)
		}
		_, err := os.ReadFile(wavFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIEEE754Extended benchmarks the 80-bit float parser.
func BenchmarkIEEE754Extended(b *testing.B) {
	// Encode 22050.0 as 80-bit extended
	buf := encodeIEEE754Extended(22050.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseIEEE754Extended(buf)
	}
}

// generateRealAIFF generates a real AIFF-C file using say, or falls back to synthetic.
func generateRealAIFF(b *testing.B) []byte {
	b.Helper()
	if _, err := exec.LookPath("say"); err == nil {
		tmp, _ := os.CreateTemp("", "bench_*.aiff")
		defer os.Remove(tmp.Name())
		tmp.Close()
		cmd := exec.Command("say", "-o", tmp.Name(), "Hello world, this is a benchmark test for audio conversion performance.")
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Logf("say failed, using synthetic: %v: %s", err, out)
			return buildSyntheticAIFFC(22050, 22050) // 1 second
		}
		data, _ := os.ReadFile(tmp.Name())
		return data
	}
	return buildSyntheticAIFFC(22050, 22050)
}

// buildSyntheticAIFFC creates a synthetic AIFF-C file with twos compression.
func buildSyntheticAIFFC(sampleRate, numSamples int) []byte {
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = int16(16000 * math.Sin(2*math.Pi*440*float64(i)/float64(sampleRate)))
	}
	return buildTestAIFFC(sampleRate, samples)
}

// encodeIEEE754Extended encodes a float64 as 80-bit IEEE 754 extended (for test helper reuse).
func encodeIEEE754ExtendedBench(f float64) []byte {
	b := make([]byte, 10)
	if f == 0 {
		return b
	}
	sign := 0
	if f < 0 {
		sign = 1
		f = -f
	}
	frac, exp := math.Frexp(f)
	exponent := exp + 16383 - 1
	mantissa := uint64(frac * (1 << 64))
	b[0] = byte((sign << 7) | (exponent >> 8))
	b[1] = byte(exponent)
	binary.BigEndian.PutUint64(b[2:], mantissa)
	return b
}
