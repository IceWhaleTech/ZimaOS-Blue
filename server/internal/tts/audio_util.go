package tts

import (
	"bytes"
	"encoding/binary"
	"math"
)

// pcmToWav converts raw PCM bytes (16-bit signed LE, mono) to WAV format.
func pcmToWav(pcmData []byte, sampleRate int) []byte {
	dataSize := len(pcmData)
	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))           // chunk size
	binary.Write(buf, binary.LittleEndian, uint16(1))            // audio format (PCM)
	binary.Write(buf, binary.LittleEndian, uint16(1))            // num channels (mono)
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))   // sample rate
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(buf, binary.LittleEndian, uint16(2))            // block align
	binary.Write(buf, binary.LittleEndian, uint16(16))           // bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcmData)

	return buf.Bytes()
}

// float32ToWav converts float32 audio samples (-1.0 to 1.0) to WAV format.
func float32ToWav(samples []float32, sampleRate int) []byte {
	// Convert float32 samples to PCM16 bytes
	pcm := make([]byte, len(samples)*2)
	for i, s := range samples {
		// Clamp to [-1, 1]
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		val := int16(s * math.MaxInt16)
		pcm[i*2] = byte(val)
		pcm[i*2+1] = byte(val >> 8)
	}
	return pcmToWav(pcm, sampleRate)
}
