// +build !windows

package windows

import (
	"context"
	"fmt"
)

// WindowsASRProvider stub for non-Windows platforms
type WindowsASRProvider struct{}

func NewWindowsASRProvider(language string) *WindowsASRProvider {
	return nil
}

func (p *WindowsASRProvider) Recognize(ctx context.Context, req *RecognizeRequest) (*RecognizeResponse, error) {
	return nil, fmt.Errorf("Windows ASR not available on this platform")
}

func (p *WindowsASRProvider) Close() {}

// GetInstalledLanguages stub for non-Windows platforms
func GetInstalledLanguages() []string {
	return []string{}
}
