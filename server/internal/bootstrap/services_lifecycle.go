package bootstrap

func (s *Services) Close() {
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
	if s.DBConn != nil {
		_ = s.DBConn.Close()
		return
	}
	if s.DB != nil {
		_ = s.DB.Close()
	}
}
