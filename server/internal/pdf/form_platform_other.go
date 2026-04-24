//go:build !darwin

package pdf

func pdfFormSupportAvailable() bool {
	return true
}
