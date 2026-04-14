package bootstrap

func closeServiceExtras(s *Services) {
	if s.APIKeyService != nil {
		s.APIKeyService.Close()
	}
	if s.PDFService != nil {
		_ = s.PDFService.Close()
	}
	if s.OCRService != nil {
		_ = s.OCRService.Close()
	}
	if s.MemoryStore != nil {
		_ = s.MemoryStore.Close()
	}
}
