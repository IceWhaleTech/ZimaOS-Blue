package bootstrap

func (s *Services) Close() {
	closeServiceExtras(s)
	if s.RuntimeDBConn != nil {
		_ = s.RuntimeDBConn.Close()
	}
	if s.DBConn != nil {
		_ = s.DBConn.Close()
		return
	}
	if s.DB != nil {
		_ = s.DB.Close()
	}
}
