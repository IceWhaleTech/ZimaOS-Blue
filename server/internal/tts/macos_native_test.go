//go:build darwin

package tts

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"testing"
)

// buildTestAIFFC creates a minimal AIFF-C (sowt) file with a sine wave.
func buildTestAIFFC(sampleRate int, samples []int16) []byte {
	numFrames := len(samples)
	ssndSize := 8 + numFrames*2 // offset(4) + blockSize(4) + PCM data

	// COMM chunk for AIFF-C: 26 bytes + compression name
	compressionName := []byte("not compressed\x00") // Pascal string (len prefix added below)
	commPayload := new(bytes.Buffer)
	binary.Write(commPayload, binary.BigEndian, int16(1))            // numChannels
	binary.Write(commPayload, binary.BigEndian, uint32(numFrames))   // numSampleFrames
	binary.Write(commPayload, binary.BigEndian, int16(16))           // sampleSize
	commPayload.Write(encodeIEEE754Extended(float64(sampleRate)))    // sampleRate (80-bit)
	commPayload.Write([]byte("sowt"))                                // compressionType
	commPayload.WriteByte(byte(len(compressionName) - 1))            // Pascal string length
	commPayload.Write(compressionName)
	if commPayload.Len()%2 != 0 {
		commPayload.WriteByte(0) // pad to even
	}

	// Build full file
	buf := new(bytes.Buffer)
	formSize := 4 + (8 + commPayload.Len()) + (8 + ssndSize)
	buf.WriteString("FORM")
	binary.Write(buf, binary.BigEndian, uint32(formSize))
	buf.WriteString("AIFC")

	// COMM chunk
	buf.WriteString("COMM")
	binary.Write(buf, binary.BigEndian, uint32(commPayload.Len()))
	buf.Write(commPayload.Bytes())

	// SSND chunk
	buf.WriteString("SSND")
	binary.Write(buf, binary.BigEndian, uint32(ssndSize))
	binary.Write(buf, binary.BigEndian, uint32(0)) // offset
	binary.Write(buf, binary.BigEndian, uint32(0)) // blockSize
	// sowt = little-endian int16
	for _, s := range samples {
		binary.Write(buf, binary.LittleEndian, s)
	}

	return buf.Bytes()
}

// encodeIEEE754Extended encodes a float64 as 80-bit IEEE 754 extended.
func encodeIEEE754Extended(f float64) []byte {
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
	// extended: exponent bias = 16383, mantissa has explicit integer bit
	exponent := exp + 16383 - 1
	mantissa := uint64(frac * (1 << 64))
	b[0] = byte((sign << 7) | (exponent >> 8))
	b[1] = byte(exponent)
	binary.BigEndian.PutUint64(b[2:], mantissa)
	return b
}

func TestAiffcToWav_Synthetic(t *testing.T) {
	// Generate 0.1s of 440Hz sine at 22050Hz
	const sampleRate = 22050
	const duration = 0.1
	numSamples := int(sampleRate * duration)
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = int16(16000 * math.Sin(2*math.Pi*440*float64(i)/sampleRate))
	}

	aiffData := buildTestAIFFC(sampleRate, samples)
	wavData, err := aiffcToWav(aiffData)
	if err != nil {
		t.Fatalf("aiffcToWav failed: %v", err)
	}

	// Verify WAV header
	if string(wavData[:4]) != "RIFF" {
		t.Fatal("missing RIFF header")
	}
	if string(wavData[8:12]) != "WAVE" {
		t.Fatal("missing WAVE marker")
	}

	// Check sample rate in fmt chunk
	sr := binary.LittleEndian.Uint32(wavData[24:28])
	if sr != sampleRate {
		t.Errorf("sample rate = %d, want %d", sr, sampleRate)
	}

	// Check data size matches
	dataSize := binary.LittleEndian.Uint32(wavData[40:44])
	if int(dataSize) != numSamples*2 {
		t.Errorf("data size = %d, want %d", dataSize, numSamples*2)
	}

	// Verify PCM samples match (sowt = already LE, should be identical)
	for i := 0; i < min(10, numSamples); i++ {
		got := int16(binary.LittleEndian.Uint16(wavData[44+i*2:]))
		if got != samples[i] {
			t.Errorf("sample[%d] = %d, want %d", i, got, samples[i])
		}
	}
}

func TestAiffcToWav_RealSay(t *testing.T) {
	if _, err := exec.LookPath("say"); err != nil {
		t.Skip("say command not available")
	}

	// Generate AIFF with say
	tmpFile, err := os.CreateTemp("", "tts_test_*.aiff")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cmd := exec.Command("say", "-o", tmpFile.Name(), "hello")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("say failed: %v: %s", err, out)
	}

	aiffData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Convert with our function
	wavData, err := aiffcToWav(aiffData)
	if err != nil {
		t.Fatalf("aiffcToWav failed: %v", err)
	}

	if string(wavData[:4]) != "RIFF" {
		t.Fatal("missing RIFF header")
	}
	if len(wavData) < 1000 {
		t.Errorf("WAV too small: %d bytes", len(wavData))
	}

	// Compare with afconvert output
	wavFile := tmpFile.Name() + ".wav"
	defer os.Remove(wavFile)
	cmd = exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16", tmpFile.Name(), wavFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("afconvert failed: %v: %s", err, out)
	}
	refData, err := os.ReadFile(wavFile)
	if err != nil {
		t.Fatal(err)
	}

	// afconvert adds a FLLR padding chunk (~4KB), so its output is larger.
	// Compare actual PCM data size instead of total file size.
	ourDataSize := binary.LittleEndian.Uint32(wavData[40:44])
	var refDataSize uint32
	for i := 12; i+8 < len(refData); {
		id := string(refData[i : i+4])
		sz := binary.LittleEndian.Uint32(refData[i+4 : i+8])
		if id == "data" {
			refDataSize = sz
			break
		}
		i += 8 + int(sz)
	}
	if ourDataSize != refDataSize {
		t.Errorf("PCM data size mismatch: ours=%d, afconvert=%d", ourDataSize, refDataSize)
	}

	t.Logf("our WAV: %d bytes (pcm=%d), afconvert WAV: %d bytes (pcm=%d)",
		len(wavData), ourDataSize, len(refData), refDataSize)
}
