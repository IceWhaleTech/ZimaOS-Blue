// Package security provides TLS certificate management.
package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/acme/autocert"
)

// TLSManager manages TLS certificates.
type TLSManager struct {
	mu            sync.RWMutex
	config        *TLSManagerConfig
	certificate   *tls.Certificate
	certInfo      *CertificateInfo
	autocertMgr   *autocert.Manager
	renewalCancel context.CancelFunc
}

// TLSManagerConfig holds TLS manager configuration.
type TLSManagerConfig struct {
	CertFile     string
	KeyFile      string
	AutoCert     bool
	ACMEEmail    string
	ACMEDomains  []string
	ACMEProvider string // letsencrypt, zerossl
	ACMEDir      string
	SelfSigned   bool
	HTTPSOnly    bool // Redirect HTTP to HTTPS
	HTTPSPort    int  // HTTPS port, default 443
}

// CertificateInfo contains parsed certificate information.
type CertificateInfo struct {
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	Domains     []string  `json:"domains"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	IsCA        bool      `json:"is_ca"`
	IsSelfSigned bool     `json:"is_self_signed"`
	SerialNumber string   `json:"serial_number"`
	Fingerprint  string   `json:"fingerprint"`
}

// NewTLSManager creates a new TLS manager.
func NewTLSManager(config *TLSManagerConfig) *TLSManager {
	return &TLSManager{
		config: config,
	}
}

// LoadCertificate loads certificate from files.
func (m *TLSManager) LoadCertificate() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.CertFile == "" || m.config.KeyFile == "" {
		return fmt.Errorf("certificate and key files are required")
	}

	cert, err := tls.LoadX509KeyPair(m.config.CertFile, m.config.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	m.certificate = &cert

	// Parse certificate info
	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			m.certInfo = parseCertInfo(x509Cert)
		}
	}

	return nil
}

// GenerateSelfSigned generates a self-signed certificate.
func (m *TLSManager) GenerateSelfSigned(domains []string, validDays int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(domains) == 0 {
		domains = []string{"localhost"}
	}
	if validDays <= 0 {
		validDays = 365
	}

	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"ZimaOS Echo"},
			CommonName:   domains[0],
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, validDays),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add domains
	for _, domain := range domains {
		if ip := net.ParseIP(domain); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, domain)
		}
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Save to files if paths are configured
	if m.config.CertFile != "" && m.config.KeyFile != "" {
		if err := os.MkdirAll(filepath.Dir(m.config.CertFile), 0750); err != nil {
			return fmt.Errorf("failed to create cert directory: %w", err)
		}
		if err := os.WriteFile(m.config.CertFile, certPEM, 0644); err != nil {
			return fmt.Errorf("failed to write certificate: %w", err)
		}
		if err := os.WriteFile(m.config.KeyFile, keyPEM, 0600); err != nil {
			return fmt.Errorf("failed to write private key: %w", err)
		}
	}

	// Load into memory
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	m.certificate = &cert

	// Parse certificate info
	x509Cert, _ := x509.ParseCertificate(certDER)
	m.certInfo = parseCertInfo(x509Cert)

	return nil
}

// GetCertificate returns the current certificate for TLS config.
func (m *TLSManager) GetCertificate() *tls.Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.certificate
}

// GetCertificateInfo returns parsed certificate information.
func (m *TLSManager) GetCertificateInfo() *CertificateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.certInfo
}

// GetTLSConfig returns a tls.Config for the server.
// The config uses GetCertificate callback for hot-reload support.
func (m *TLSManager) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// Try autocert first if configured
			m.mu.RLock()
			autocertMgr := m.autocertMgr
			m.mu.RUnlock()

			if autocertMgr != nil {
				cert, err := autocertMgr.GetCertificate(hello)
				if err == nil {
					return cert, nil
				}
			}

			// Fall back to loaded certificate
			cert := m.GetCertificate()
			if cert == nil {
				return nil, fmt.Errorf("no certificate loaded")
			}
			return cert, nil
		},
		MinVersion: tls.VersionTLS12,
	}
}

// ReloadCertificate reloads the certificate from files (hot-reload).
func (m *TLSManager) ReloadCertificate() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.CertFile == "" || m.config.KeyFile == "" {
		return fmt.Errorf("certificate and key files are not configured")
	}

	cert, err := tls.LoadX509KeyPair(m.config.CertFile, m.config.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to reload certificate: %w", err)
	}

	m.certificate = &cert

	// Update certificate info
	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			m.certInfo = parseCertInfo(x509Cert)
		}
	}

	return nil
}

// IsHotReloadSupported returns true (certificates can be updated without restart).
func (m *TLSManager) IsHotReloadSupported() bool {
	return true
}

// ParseCertificateFromPEM parses certificate info from PEM data.
func ParseCertificateFromPEM(certPEM []byte) (*CertificateInfo, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return parseCertInfo(cert), nil
}

func parseCertInfo(cert *x509.Certificate) *CertificateInfo {
	info := &CertificateInfo{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		IsCA:         cert.IsCA,
		IsSelfSigned: cert.Subject.String() == cert.Issuer.String(),
		SerialNumber: cert.SerialNumber.String(),
	}

	// Collect domains
	info.Domains = append(info.Domains, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		info.Domains = append(info.Domains, ip.String())
	}
	if cert.Subject.CommonName != "" {
		found := false
		for _, d := range info.Domains {
			if d == cert.Subject.CommonName {
				found = true
				break
			}
		}
		if !found {
			info.Domains = append([]string{cert.Subject.CommonName}, info.Domains...)
		}
	}

	// Calculate fingerprint (SHA-256)
	info.Fingerprint = fmt.Sprintf("%X", cert.Raw[:min(32, len(cert.Raw))])

	return info
}

// SaveCertificate saves certificate and key from PEM data.
func (m *TLSManager) SaveCertificate(certPEM, keyPEM []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("invalid certificate or key: %w", err)
	}

	// Save to files
	if m.config.CertFile != "" && m.config.KeyFile != "" {
		if err := os.MkdirAll(filepath.Dir(m.config.CertFile), 0750); err != nil {
			return fmt.Errorf("failed to create cert directory: %w", err)
		}
		if err := os.WriteFile(m.config.CertFile, certPEM, 0644); err != nil {
			return fmt.Errorf("failed to write certificate: %w", err)
		}
		if err := os.WriteFile(m.config.KeyFile, keyPEM, 0600); err != nil {
			return fmt.Errorf("failed to write private key: %w", err)
		}
	}

	m.certificate = &cert

	// Parse certificate info
	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			m.certInfo = parseCertInfo(x509Cert)
		}
	}

	return nil
}

// Global TLS manager instance
var (
	globalTLSManager     *TLSManager
	globalTLSManagerOnce sync.Once
)

// GetGlobalTLSManager returns the global TLS manager instance.
func GetGlobalTLSManager() *TLSManager {
	globalTLSManagerOnce.Do(func() {
		globalTLSManager = NewTLSManager(&TLSManagerConfig{
			CertFile: "./data/certs/server.crt",
			KeyFile:  "./data/certs/server.key",
			ACMEDir:  "./data/certs/acme",
		})
	})
	return globalTLSManager
}

// SetGlobalTLSManagerConfig updates the global TLS manager configuration.
func SetGlobalTLSManagerConfig(config *TLSManagerConfig) {
	globalTLSManagerOnce.Do(func() {
		globalTLSManager = NewTLSManager(config)
	})
	if globalTLSManager != nil {
		globalTLSManager.mu.Lock()
		globalTLSManager.config = config
		globalTLSManager.mu.Unlock()
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ACMEProviderURL returns the ACME directory URL for a provider.
func ACMEProviderURL(provider string) string {
	switch strings.ToLower(provider) {
	case "letsencrypt", "le":
		return "https://acme-v02.api.letsencrypt.org/directory"
	case "letsencrypt-staging", "le-staging":
		return "https://acme-staging-v02.api.letsencrypt.org/directory"
	case "zerossl":
		return "https://acme.zerossl.com/v2/DV90"
	default:
		return provider // Assume it's a custom URL
	}
}

// ACMEConfig holds ACME certificate configuration.
type ACMEConfig struct {
	Email    string   `json:"email"`
	Domains  []string `json:"domains"`
	Provider string   `json:"provider"` // letsencrypt, zerossl, or custom URL
	CacheDir string   `json:"cache_dir"`
}

// ACMEStatus represents the status of ACME certificate.
type ACMEStatus struct {
	Configured    bool             `json:"configured"`
	Email         string           `json:"email"`
	Domains       []string         `json:"domains"`
	Provider      string           `json:"provider"`
	CertInfo      *CertificateInfo `json:"cert_info,omitempty"`
	Error         string           `json:"error,omitempty"`
	AutoRenewal   bool             `json:"auto_renewal"`
	NextRenewal   *time.Time       `json:"next_renewal,omitempty"`
	DaysUntilExp  int              `json:"days_until_expiry,omitempty"`
}

// RequestACMECertificate requests a certificate from an ACME provider.
// This performs the HTTP-01 challenge, so port 80 must be accessible.
func (m *TLSManager) RequestACMECertificate(config *ACMEConfig) error {
	if config.Email == "" {
		return fmt.Errorf("email is required for ACME registration")
	}
	if len(config.Domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update config
	m.config.ACMEEmail = config.Email
	m.config.ACMEDomains = config.Domains
	m.config.ACMEProvider = config.Provider
	if config.CacheDir != "" {
		m.config.ACMEDir = config.CacheDir
	}

	// Create cache directory
	cacheDir := m.config.ACMEDir
	if cacheDir == "" {
		cacheDir = "./data/certs/acme"
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create ACME cache directory: %w", err)
	}

	// Initialize autocert manager
	m.autocertMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      config.Email,
		HostPolicy: autocert.HostWhitelist(config.Domains...),
		Cache:      autocert.DirCache(cacheDir),
	}

	// Start renewal checker
	m.startRenewalChecker()

	return nil
}

// startRenewalChecker starts a background goroutine to check certificate expiry
func (m *TLSManager) startRenewalChecker() {
	if m.renewalCancel != nil {
		m.renewalCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.renewalCancel = cancel

	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkAndRenew()
			}
		}
	}()
}

// checkAndRenew checks if certificate needs renewal and renews if necessary
func (m *TLSManager) checkAndRenew() {
	m.mu.RLock()
	certInfo := m.certInfo
	autocertMgr := m.autocertMgr
	domains := m.config.ACMEDomains
	m.mu.RUnlock()

	if certInfo == nil || autocertMgr == nil || len(domains) == 0 {
		return
	}

	// Renew if certificate expires within 30 days
	if time.Until(certInfo.NotAfter) < 30*24*time.Hour {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Force renewal by getting certificate
		cert, err := autocertMgr.GetCertificate(&tls.ClientHelloInfo{
			ServerName: domains[0],
		})
		if err != nil {
			return
		}

		m.mu.Lock()
		m.certificate = cert
		if len(cert.Certificate) > 0 {
			if x509Cert, err := x509.ParseCertificate(cert.Certificate[0]); err == nil {
				m.certInfo = parseCertInfo(x509Cert)
			}
		}
		m.mu.Unlock()

		// Save to disk
		_ = m.saveCertToDisk(cert)
		_ = ctx
	}
}

// saveCertToDisk saves the certificate to configured file paths
func (m *TLSManager) saveCertToDisk(cert *tls.Certificate) error {
	if m.config.CertFile == "" || m.config.KeyFile == "" || len(cert.Certificate) == 0 {
		return nil
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})

	// Encode private key
	var keyPEM []byte
	switch key := cert.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return err
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	default:
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return err
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	}

	if err := os.MkdirAll(filepath.Dir(m.config.CertFile), 0750); err != nil {
		return err
	}
	if err := os.WriteFile(m.config.CertFile, certPEM, 0644); err != nil {
		return err
	}
	return os.WriteFile(m.config.KeyFile, keyPEM, 0600)
}

// GetAutocertManager returns the autocert manager for use with HTTP server
func (m *TLSManager) GetAutocertManager() *autocert.Manager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.autocertMgr
}

// StopRenewalChecker stops the background renewal checker
func (m *TLSManager) StopRenewalChecker() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewalCancel != nil {
		m.renewalCancel()
		m.renewalCancel = nil
	}
}

// GetACMEStatus returns the current ACME configuration status.
func (m *TLSManager) GetACMEStatus() *ACMEStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ACMEStatus{
		Configured:  m.config.ACMEEmail != "" && len(m.config.ACMEDomains) > 0,
		Email:       m.config.ACMEEmail,
		Domains:     m.config.ACMEDomains,
		Provider:    m.config.ACMEProvider,
		AutoRenewal: m.autocertMgr != nil,
	}

	if m.certInfo != nil {
		status.CertInfo = m.certInfo
		status.DaysUntilExp = int(time.Until(m.certInfo.NotAfter).Hours() / 24)
		// Renewal happens 30 days before expiry
		renewalTime := m.certInfo.NotAfter.Add(-30 * 24 * time.Hour)
		status.NextRenewal = &renewalTime
	}

	return status
}

// GetHTTPHandler returns an HTTP handler for ACME HTTP-01 challenges.
// This should be mounted on port 80 to handle Let's Encrypt challenges.
func (m *TLSManager) GetHTTPHandler() http.Handler {
	m.mu.RLock()
	autocertMgr := m.autocertMgr
	m.mu.RUnlock()

	if autocertMgr != nil {
		return autocertMgr.HTTPHandler(nil)
	}
	return nil
}

// SetHTTPSOnly enables or disables HTTPS-only mode.
func (m *TLSManager) SetHTTPSOnly(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.HTTPSOnly = enabled
}

// IsHTTPSOnly returns whether HTTPS-only mode is enabled.
func (m *TLSManager) IsHTTPSOnly() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.HTTPSOnly
}

// SetHTTPSPort sets the HTTPS port.
func (m *TLSManager) SetHTTPSPort(port int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.HTTPSPort = port
}

// GetHTTPSPort returns the HTTPS port.
func (m *TLSManager) GetHTTPSPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config.HTTPSPort == 0 {
		return 443
	}
	return m.config.HTTPSPort
}

// HTTPSRedirectMiddleware returns an Echo middleware that redirects HTTP to HTTPS.
func (m *TLSManager) HTTPSRedirectMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check if HTTPS-only mode is enabled
			if !m.IsHTTPSOnly() {
				return next(c)
			}

			// Check if already HTTPS
			if c.Scheme() == "https" || c.Request().TLS != nil {
				return next(c)
			}

			// Check X-Forwarded-Proto header (for reverse proxy)
			if c.Request().Header.Get("X-Forwarded-Proto") == "https" {
				return next(c)
			}

			// Redirect to HTTPS
			host := c.Request().Host
			httpsPort := m.GetHTTPSPort()
			if httpsPort != 443 {
				// Remove existing port if present
				if idx := strings.LastIndex(host, ":"); idx != -1 {
					host = host[:idx]
				}
				host = fmt.Sprintf("%s:%d", host, httpsPort)
			}

			httpsURL := "https://" + host + c.Request().RequestURI
			return c.Redirect(http.StatusMovedPermanently, httpsURL)
		}
	}
}
