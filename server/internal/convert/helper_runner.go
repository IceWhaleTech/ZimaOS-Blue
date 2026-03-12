package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type HelperRunner struct{}

func NewHelperRunner() *HelperRunner {
	return &HelperRunner{}
}

func (r *HelperRunner) Status() HelperStatus {
	path, mode, err := r.resolve()
	if err != nil {
		return HelperStatus{Available: false, Error: err.Error()}
	}
	return HelperStatus{Available: true, Mode: mode, Path: path}
}

func (r *HelperRunner) Run(ctx context.Context, payload map[string]interface{}) (*helperResponse, error) {
	helperPath, mode, err := r.resolve()
	if err != nil {
		return nil, err
	}
	requestFile, err := os.CreateTemp("", "blue-convert-helper-*.json")
	if err != nil {
		return nil, fmt.Errorf("create helper request: %w", err)
	}
	requestPath := requestFile.Name()
	defer os.Remove(requestPath)
	defer requestFile.Close()
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode helper payload: %w", err)
	}
	if _, err := requestFile.Write(encoded); err != nil {
		return nil, fmt.Errorf("write helper payload: %w", err)
	}
	if err := requestFile.Close(); err != nil {
		return nil, fmt.Errorf("close helper payload: %w", err)
	}

	var cmd *exec.Cmd
	if mode == "swift" {
		cmd = exec.CommandContext(ctx, "swift", helperPath, requestPath)
	} else {
		cmd = exec.CommandContext(ctx, helperPath, requestPath)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("convert helper failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	var response helperResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse helper response: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("convert helper: %s", response.Error)
	}
	return &response, nil
}

func (r *HelperRunner) resolve() (string, string, error) {
	if envPath := strings.TrimSpace(os.Getenv("BLUE_CONVERT_HELPER")); envPath != "" {
		if info, err := os.Stat(envPath); err == nil && !info.IsDir() {
			return envPath, "binary", nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, "blue-convert-helper"),
			filepath.Join(exeDir, "..", "Resources", "blue-convert-helper"),
			filepath.Join(exeDir, "..", "..", "Resources", "blue-convert-helper"),
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, "binary", nil
			}
		}
	}
	if _, err := exec.LookPath("swift"); err != nil {
		return "", "", fmt.Errorf("swift helper unavailable: %w", err)
	}
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "server", "internal", "convert", "helper", "blue-convert-helper.swift"),
		filepath.Join(wd, "internal", "convert", "helper", "blue-convert-helper.swift"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, "swift", nil
		}
	}
	return "", "", fmt.Errorf("blue-convert-helper not found")
}
