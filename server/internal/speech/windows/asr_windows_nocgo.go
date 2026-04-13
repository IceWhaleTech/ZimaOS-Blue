//go:build windows && !cgo

package windows

import (
	"context"
	"fmt"
)

// WindowsASRProvider is a no-cgo fallback for Windows-targeted builds.
type WindowsASRProvider struct{}

func NewWindowsASRProvider(language string) *WindowsASRProvider {
	return nil
}

func (p *WindowsASRProvider) Recognize(ctx context.Context, req *RecognizeRequest) (*RecognizeResponse, error) {
	return nil, fmt.Errorf("Windows ASR requires cgo in this build")
}

func (p *WindowsASRProvider) Close() {}

func GetInstalledLanguages() []string {
	return []string{}
}
