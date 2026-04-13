//go:build windows && !cgo

package windows

import "testing"

func TestWindowsProviders_NoCGOFallbacks(t *testing.T) {
	if provider := NewWindowsTTSProvider(); provider != nil {
		t.Fatalf("NewWindowsTTSProvider() = %#v, want nil without cgo", provider)
	}
	if provider := NewWindowsASRProvider("en-US"); provider != nil {
		t.Fatalf("NewWindowsASRProvider() = %#v, want nil without cgo", provider)
	}
	if languages := GetInstalledLanguages(); len(languages) != 0 {
		t.Fatalf("GetInstalledLanguages() = %+v, want empty without cgo", languages)
	}
}
