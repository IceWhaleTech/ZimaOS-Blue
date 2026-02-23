package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// route53Provider is a lightweight Route53 DNS-01 provider that replaces
// the aws-sdk-go-v2 dependency (56 packages) with ~150 lines of stdlib code.
// It only needs ChangeResourceRecordSets for ACME DNS-01 challenges.
type route53Provider struct {
	accessKey string
	secretKey string
	region    string
	hostedID  string
}

func newRoute53Provider() (*route53Provider, error) {
	ak := os.Getenv("AWS_ACCESS_KEY_ID")
	sk := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_REGION")
	zone := os.Getenv("AWS_HOSTED_ZONE_ID")
	if ak == "" || sk == "" {
		return nil, fmt.Errorf("route53: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY required")
	}
	if region == "" {
		region = "us-east-1"
	}
	if zone == "" {
		return nil, fmt.Errorf("route53: AWS_HOSTED_ZONE_ID required")
	}
	return &route53Provider{accessKey: ak, secretKey: sk, region: region, hostedID: zone}, nil
}

func (p *route53Provider) Present(domain, token, keyAuth string) error {
	fqdn := "_acme-challenge." + domain + "."
	return p.changeRecord("UPSERT", fqdn, keyAuth)
}

func (p *route53Provider) CleanUp(domain, token, keyAuth string) error {
	fqdn := "_acme-challenge." + domain + "."
	return p.changeRecord("DELETE", fqdn, keyAuth)
}

func (p *route53Provider) changeRecord(action, fqdn, value string) error {
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ChangeResourceRecordSetsRequest xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ChangeBatch>
    <Changes>
      <Change>
        <Action>%s</Action>
        <ResourceRecordSet>
          <Name>%s</Name>
          <Type>TXT</Type>
          <TTL>60</TTL>
          <ResourceRecords>
            <ResourceRecord><Value>"%s"</Value></ResourceRecord>
          </ResourceRecords>
        </ResourceRecordSet>
      </Change>
    </Changes>
  </ChangeBatch>
</ChangeResourceRecordSetsRequest>`, action, fqdn, value)

	url := fmt.Sprintf("https://route53.amazonaws.com/2013-04-01/hostedzone/%s/rrset", p.hostedID)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")

	p.signV4(req, []byte(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("route53: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		var errResp struct {
			Message string `xml:"Message"`
		}
		xml.Unmarshal(b, &errResp)
		return fmt.Errorf("route53: %s (HTTP %d)", errResp.Message, resp.StatusCode)
	}
	return nil
}

// signV4 signs the request using AWS Signature Version 4.
// Route53 is a global service but uses us-east-1 for signing.
func (p *route53Provider) signV4(req *http.Request, payload []byte) {
	now := timeutil.NowTime().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	region := "us-east-1" // Route53 always signs with us-east-1
	service := "route53"

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.URL.Host)

	payloadHash := sha256Hex(payload)

	// Canonical request
	canonHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"), req.URL.Host, amzDate)
	signedHeaders := "content-type;host;x-amz-date"

	canonReq := strings.Join([]string{
		req.Method,
		req.URL.Path,
		"", // no query string
		canonHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// String to sign
	scope := fmt.Sprintf("%s/%s/%s/aws4_request", date, region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, scope, sha256Hex([]byte(canonReq)))

	// Signing key
	kDate := hmacSHA256([]byte("AWS4"+p.secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	sig := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.accessKey, scope, signedHeaders, sig))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Ensure route53Provider satisfies the dnsProvider interface.
var _ dnsProvider = (*route53Provider)(nil)
