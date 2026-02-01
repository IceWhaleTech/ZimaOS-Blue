package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	home, _ := os.UserHomeDir()
	modelDir := filepath.Join(home, ".local", "share", "zimaos-echo", "sherpa-tts")
	binDir := filepath.Join(modelDir, "bin")
	modelPath := filepath.Join(modelDir, "kokoro-en-v0_19")

	exePath := filepath.Join(binDir, "sherpa-onnx-offline-tts.exe")

	// Check exe exists
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		fmt.Println("TTS exe not found:", exePath)
		fmt.Println("Please copy from:", filepath.Join(modelDir, "lib", "sherpa-onnx-v1.12.23-win-x64-shared", "bin"))
		return
	}

	// Check model exists
	if _, err := os.Stat(filepath.Join(modelPath, "model.onnx")); os.IsNotExist(err) {
		fmt.Println("Model not found:", modelPath)
		return
	}

	outputPath := filepath.Join(os.TempDir(), "test-tts.wav")

	args := []string{
		"--kokoro-model=" + filepath.Join(modelPath, "model.onnx"),
		"--kokoro-voices=" + filepath.Join(modelPath, "voices.bin"),
		"--kokoro-tokens=" + filepath.Join(modelPath, "tokens.txt"),
		"--kokoro-data-dir=" + filepath.Join(modelPath, "espeak-ng-data"),
		"--sid=0",
		"--output-filename=" + outputPath,
		"--text=Hello, this is a test of the text to speech system.",
	}

	fmt.Println("Running:", exePath)
	fmt.Println("Args:", args)

	cmd := exec.Command(exePath, args...)
	cmd.Dir = binDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Check output
	info, err := os.Stat(outputPath)
	if err != nil {
		fmt.Println("Output file error:", err)
		return
	}

	fmt.Println("Success! Output:", outputPath, "Size:", info.Size())
}
