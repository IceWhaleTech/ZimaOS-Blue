package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// dnsProvider is the interface for DNS-01 challenge providers.
type dnsProvider interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}

// newDNSProvider creates a DNS provider by name.
func newDNSProvider(name string) (dnsProvider, error) {
	switch strings.ToLower(name) {
	case "cloudflare":
		return newCloudflareDNSProvider()
	case "route53":
		return newRoute53Provider()
	case "godaddy":
		return newGoDaddyDNSProvider()
	case "namecheap":
		return newNamecheapDNSProvider()
	case "alidns", "aliyun":
		return newAliDNSProvider()
	case "tencentcloud", "dnspod":
		return newDNSPodProvider()
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", name)
	}
}

var dnsHTTP = &http.Client{Timeout: 30 * time.Second}

// --- Cloudflare DNS ---

type cloudflareDNS struct {
	apiToken string
	zoneID   string
}

func newCloudflareDNSProvider() (*cloudflareDNS, error) {
	token := os.Getenv("CF_DNS_API_TOKEN")
	if token == "" {
		token = os.Getenv("CLOUDFLARE_DNS_API_TOKEN")
	}
	zone := os.Getenv("CF_ZONE_ID")
	if zone == "" {
		zone = os.Getenv("CLOUDFLARE_ZONE_ID")
	}
	if token == "" {
		return nil, fmt.Errorf("cloudflare: CF_DNS_API_TOKEN required")
	}
	if zone == "" {
		return nil, fmt.Errorf("cloudflare: CF_ZONE_ID required")
	}
	return &cloudflareDNS{apiToken: token, zoneID: zone}, nil
}

func (p *cloudflareDNS) Present(domain, token, keyAuth string) error {
	fqdn := "_acme-challenge." + domain
	body, _ := json.Marshal(map[string]interface{}{
		"type":    "TXT",
		"name":    fqdn,
		"content": keyAuth,
		"ttl":     60,
	})
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", p.zoneID),
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := dnsHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare: HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (p *cloudflareDNS) CleanUp(domain, token, keyAuth string) error {
	fqdn := "_acme-challenge." + domain
	// Find record ID
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=TXT&name=%s", p.zoneID, fqdn), nil)
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	resp, err := dnsHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result struct {
		Result []struct{ ID string `json:"id"` } `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	for _, r := range result.Result {
		req, _ := http.NewRequest("DELETE",
			fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", p.zoneID, r.ID), nil)
		req.Header.Set("Authorization", "Bearer "+p.apiToken)
		dnsHTTP.Do(req)
	}
	return nil
}

// --- GoDaddy DNS ---

type godaddyDNS struct {
	apiKey    string
	apiSecret string
}

func newGoDaddyDNSProvider() (*godaddyDNS, error) {
	key := os.Getenv("GODADDY_API_KEY")
	secret := os.Getenv("GODADDY_API_SECRET")
	if key == "" || secret == "" {
		return nil, fmt.Errorf("godaddy: GODADDY_API_KEY and GODADDY_API_SECRET required")
	}
	return &godaddyDNS{apiKey: key, apiSecret: secret}, nil
}

func (p *godaddyDNS) Present(domain, token, keyAuth string) error {
	// GoDaddy API: PUT /v1/domains/{domain}/records/TXT/_acme-challenge
	body, _ := json.Marshal([]map[string]interface{}{
		{"data": keyAuth, "ttl": 600},
	})
	req, _ := http.NewRequest("PUT",
		fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records/TXT/_acme-challenge", domain),
		bytes.NewReader(body))
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", p.apiKey, p.apiSecret))
	req.Header.Set("Content-Type", "application/json")
	resp, err := dnsHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("godaddy: HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (p *godaddyDNS) CleanUp(domain, token, keyAuth string) error {
	// GoDaddy: DELETE /v1/domains/{domain}/records/TXT/_acme-challenge
	req, _ := http.NewRequest("DELETE",
		fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records/TXT/_acme-challenge", domain), nil)
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", p.apiKey, p.apiSecret))
	dnsHTTP.Do(req)
	return nil
}

// --- Namecheap DNS ---

type namecheapDNS struct {
	apiUser string
	apiKey  string
	clientIP string
}

func newNamecheapDNSProvider() (*namecheapDNS, error) {
	user := os.Getenv("NAMECHEAP_API_USER")
	key := os.Getenv("NAMECHEAP_API_KEY")
	if user == "" || key == "" {
		return nil, fmt.Errorf("namecheap: NAMECHEAP_API_USER and NAMECHEAP_API_KEY required")
	}
	ip := os.Getenv("NAMECHEAP_CLIENT_IP")
	if ip == "" {
		ip = "127.0.0.1"
	}
	return &namecheapDNS{apiUser: user, apiKey: key, clientIP: ip}, nil
}

func (p *namecheapDNS) Present(domain, token, keyAuth string) error {
	// Namecheap uses XML API — simplified: set TXT host record
	url := fmt.Sprintf("https://api.namecheap.com/xml.response?ApiUser=%s&ApiKey=%s&UserName=%s&ClientIp=%s&Command=namecheap.domains.dns.setHosts&SLD=%s&TLD=%s&HostName1=_acme-challenge&RecordType1=TXT&Address1=%s&TTL1=60",
		p.apiUser, p.apiKey, p.apiUser, p.clientIP, sld(domain), tld(domain), keyAuth)
	resp, err := dnsHTTP.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (p *namecheapDNS) CleanUp(domain, token, keyAuth string) error {
	return nil // Namecheap overwrites on next Present; no explicit delete needed
}

// --- Alibaba Cloud DNS ---

type alidnsDNS struct {
	accessKey string
	secretKey string
}

func newAliDNSProvider() (*alidnsDNS, error) {
	ak := os.Getenv("ALICLOUD_ACCESS_KEY")
	sk := os.Getenv("ALICLOUD_SECRET_KEY")
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("alidns: ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY required")
	}
	return &alidnsDNS{accessKey: ak, secretKey: sk}, nil
}

func (p *alidnsDNS) Present(domain, token, keyAuth string) error {
	rr, rootDomain := splitDomainRR("_acme-challenge", domain)
	params := map[string]string{
		"Action":   "AddDomainRecord",
		"DomainName": rootDomain,
		"RR":       rr,
		"Type":     "TXT",
		"Value":    keyAuth,
		"TTL":      "60",
	}
	_, err := p.doAPI(params)
	return err
}

func (p *alidnsDNS) CleanUp(domain, token, keyAuth string) error {
	rr, rootDomain := splitDomainRR("_acme-challenge", domain)
	// Find the record ID first
	params := map[string]string{
		"Action":     "DescribeDomainRecords",
		"DomainName": rootDomain,
		"RRKeyWord":  rr,
		"TypeKeyWord": "TXT",
	}
	body, err := p.doAPI(params)
	if err != nil {
		return err
	}
	var result struct {
		DomainRecords struct {
			Record []struct {
				RecordId string `json:"RecordId"`
				Value    string `json:"Value"`
			} `json:"Record"`
		} `json:"DomainRecords"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("alidns: parse response: %w", err)
	}
	for _, rec := range result.DomainRecords.Record {
		if rec.Value == keyAuth {
			delParams := map[string]string{
				"Action":   "DeleteDomainRecord",
				"RecordId": rec.RecordId,
			}
			if _, err := p.doAPI(delParams); err != nil {
				return err
			}
		}
	}
	return nil
}

const alidnsEndpoint = "https://alidns.aliyuncs.com/"

// doAPI calls the Alibaba Cloud DNS API with HMAC-SHA1 signing.
func (p *alidnsDNS) doAPI(params map[string]string) ([]byte, error) {
	return alidnsDoAPIWithURL(p, alidnsEndpoint, params)
}

// alidnsDoAPIWithURL signs and sends a request to the given base URL.
func alidnsDoAPIWithURL(p *alidnsDNS, baseURL string, params map[string]string) ([]byte, error) {
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"
	params["AccessKeyId"] = p.accessKey
	params["SignatureMethod"] = "HMAC-SHA1"
	params["SignatureVersion"] = "1.0"
	if params["Timestamp"] == "" {
		params["Timestamp"] = timeutil.NowTime().UTC().Format("2006-01-02T15:04:05Z")
	}
	if params["SignatureNonce"] == "" {
		params["SignatureNonce"] = fmt.Sprintf("%d%d", timeutil.NowNano(), rand.Int())
	}

	// Build sorted query string for signing
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonParts []string
	for _, k := range keys {
		canonParts = append(canonParts, aliEncode(k)+"="+aliEncode(params[k]))
	}
	canonQuery := strings.Join(canonParts, "&")

	stringToSign := "GET&" + aliEncode("/") + "&" + aliEncode(canonQuery)
	params["Signature"] = aliHmacSHA1(p.secretKey+"&", stringToSign)

	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	reqURL := strings.TrimRight(baseURL, "/") + "/?" + vals.Encode()

	resp, err := dnsHTTP.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("alidns: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("alidns: HTTP %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

// alidnsCleanUpWithURL is a test-friendly version of CleanUp that uses a custom endpoint.
func alidnsCleanUpWithURL(p *alidnsDNS, baseURL, domain, token, keyAuth string) error {
	rr, rootDomain := splitDomainRR("_acme-challenge", domain)
	params := map[string]string{
		"Action":      "DescribeDomainRecords",
		"DomainName":  rootDomain,
		"RRKeyWord":   rr,
		"TypeKeyWord": "TXT",
	}
	body, err := alidnsDoAPIWithURL(p, baseURL, params)
	if err != nil {
		return err
	}
	var result struct {
		DomainRecords struct {
			Record []struct {
				RecordId string `json:"RecordId"`
				Value    string `json:"Value"`
			} `json:"Record"`
		} `json:"DomainRecords"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("alidns: parse response: %w", err)
	}
	for _, rec := range result.DomainRecords.Record {
		if rec.Value == keyAuth {
			delParams := map[string]string{
				"Action":   "DeleteDomainRecord",
				"RecordId": rec.RecordId,
			}
			if _, err := alidnsDoAPIWithURL(p, baseURL, delParams); err != nil {
				return err
			}
		}
	}
	return nil
}

// aliHmacSHA1 computes HMAC-SHA1 and returns base64-encoded result.
func aliHmacSHA1(key, data string) string {
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// aliEncode performs percent-encoding per Alibaba Cloud's spec (RFC 3986 with
// extra replacements: + → %20, * → %2A, %7E → ~).
func aliEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// splitDomainRR splits a prefix and domain into RR and root domain.
// e.g. ("_acme-challenge", "sub.example.com") → ("_acme-challenge.sub", "example.com")
func splitDomainRR(prefix, domain string) (string, string) {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return prefix, domain
	}
	// root = last two parts, RR = prefix + remaining subdomains
	root := strings.Join(parts[len(parts)-2:], ".")
	sub := strings.Join(parts[:len(parts)-2], ".")
	return prefix + "." + sub, root
}

// --- DNSPod (Tencent Cloud) ---

type dnspodDNS struct {
	secretID  string
	secretKey string
}

func newDNSPodProvider() (*dnspodDNS, error) {
	id := os.Getenv("TENCENTCLOUD_SECRET_ID")
	key := os.Getenv("TENCENTCLOUD_SECRET_KEY")
	if id == "" || key == "" {
		return nil, fmt.Errorf("dnspod: TENCENTCLOUD_SECRET_ID and TENCENTCLOUD_SECRET_KEY required")
	}
	return &dnspodDNS{secretID: id, secretKey: key}, nil
}

func (p *dnspodDNS) Present(domain, token, keyAuth string) error {
	// Simplified: use DNSPod API via HTTP
	// In practice this needs TC3-HMAC-SHA256 signing — placeholder for now
	return fmt.Errorf("dnspod: not yet implemented in lite mode, use full build")
}

func (p *dnspodDNS) CleanUp(domain, token, keyAuth string) error {
	return nil
}

// helpers
func sld(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return domain
}

func tld(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}
