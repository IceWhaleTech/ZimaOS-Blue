//go:build darwin

package speech

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// buildTestWAV creates a 16-bit mono PCM WAV with a sine wave.
func buildTestWAV(sampleRate, numSamples int) []byte {
	dataSize := numSamples * 2
	buf := new(bytes.Buffer)
	buf.Grow(44 + dataSize)

	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2)) // byteRate
	binary.Write(buf, binary.LittleEndian, uint16(2))            // blockAlign
	binary.Write(buf, binary.LittleEndian, uint16(16))           // bitsPerSample
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	for i := 0; i < numSamples; i++ {
		s := int16(16000 * math.Sin(2*math.Pi*440*float64(i)/float64(sampleRate)))
		binary.Write(buf, binary.LittleEndian, s)
	}
	return buf.Bytes()
}

func TestParseWAVData(t *testing.T) {
	wav := buildTestWAV(16000, 16000) // 1 second
	pcm, sr, ch, bps, err := parseWAVData(wav)
	if err != nil {
		t.Fatalf("parseWAVData: %v", err)
	}
	if sr != 16000 {
		t.Errorf("sampleRate = %d, want 16000", sr)
	}
	if ch != 1 {
		t.Errorf("channels = %d, want 1", ch)
	}
	if bps != 16 {
		t.Errorf("bitsPerSample = %d, want 16", bps)
	}
	if len(pcm) != 32000 {
		t.Errorf("pcm len = %d, want 32000", len(pcm))
	}
}

func BenchmarkParseWAVData(b *testing.B) {
	wav := buildTestWAV(16000, 16000*5) // 5 seconds
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _, err := parseWAVData(wav)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func createAndWriteTemp(data []byte, ext string) (string, error) {
	f, err := os.CreateTemp("", "bench-stt-*"+ext)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func removeTempFile(path string) { os.Remove(path) }

// BenchmarkTempFileWrite benchmarks the temp file write path (the I/O we eliminate).
func BenchmarkTempFileWrite(b *testing.B) {
	wav := buildTestWAV(16000, 16000*5) // 5 seconds of audio
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, err := createAndWriteTemp(wav, ".wav")
		if err != nil {
			b.Fatal(err)
		}
		removeTempFile(f)
	}
}

// BenchmarkTempFileRoundTrip benchmarks write + read + delete (full temp file overhead).
func BenchmarkTempFileRoundTrip(b *testing.B) {
	wav := buildTestWAV(16000, 16000*5)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		path, err := createAndWriteTemp(wav, ".wav")
		if err != nil {
			b.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
		removeTempFile(path)
	}
}

// BenchmarkBufferPrepare benchmarks parsing WAV + converting int16→float32
// (the CPU work in the buffer path, excluding the ObjC calls).
func BenchmarkBufferPrepare(b *testing.B) {
	wav := buildTestWAV(16000, 16000*5) // 5 seconds
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pcm, _, channels, _, err := parseWAVData(wav)
		if err != nil {
			b.Fatal(err)
		}
		// Simulate int16→float32 conversion
		numFrames := len(pcm) / 2 / channels
		out := make([]float32, numFrames)
		for j := 0; j < numFrames; j++ {
			idx := j * channels * 2
			s := int16(binary.LittleEndian.Uint16(pcm[idx : idx+2]))
			out[j] = float32(s) / 32768.0
		}
		_ = out
	}
}
