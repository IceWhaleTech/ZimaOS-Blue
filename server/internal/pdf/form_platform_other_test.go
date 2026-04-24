//go:build !darwin

package pdf

import "testing"

func TestPDFFormSupportAvailableOnNonDarwin(t *testing.T) {
	if !pdfFormSupportAvailable() {
		t.Fatal("expected PDF form support to remain enabled on non-darwin platforms")
	}
}
