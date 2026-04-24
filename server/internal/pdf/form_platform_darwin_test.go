//go:build darwin

package pdf

import "testing"

func TestPDFFormSupportAvailableOnDarwin(t *testing.T) {
	if !pdfFormSupportAvailable() {
		t.Fatal("expected PDF form support to be available on darwin")
	}
}
