package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAliHmacSHA1(t *testing.T) {
	got := aliHmacSHA1("key", "data")
	want := "EEFSxb/coHvGM+69RhmfAlXJ9J0="
	if got != want {
		t.Errorf("aliHmacSHA1(\"key\", \"data\") = %q, want %q", got, want)
	}
}

func TestAliEncode(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello world", "hello%20world"},
		{"test*value", "test%2Avalue"},
		{"keep~tilde", "keep~tilde"},
		{"a+b", "a%2Bb"},
		{"2006-01-02T15:04:05Z", "2006-01-02T15%3A04%3A05Z"},
		{"normal", "normal"},
	}
	for _, tt := range tests {
		if got := aliEncode(tt.input); got != tt.want {
			t.Errorf("aliEncode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitDomainRR(t *testing.T) {
	tests := []struct {
		prefix, domain, wantRR, wantRoot string
	}{
		{"_acme-challenge", "example.com", "_acme-challenge", "example.com"},
		{"_acme-challenge", "sub.example.com", "_acme-challenge.sub", "example.com"},
		{"_acme-challenge", "a.b.example.com", "_acme-challenge.a.b", "example.com"},
	}
	for _, tt := range tests {
		rr, root := splitDomainRR(tt.prefix, tt.domain)
		if rr != tt.wantRR || root != tt.wantRoot {
			t.Errorf("splitDomainRR(%q, %q) = (%q, %q), want (%q, %q)",
				tt.prefix, tt.domain, rr, root, tt.wantRR, tt.wantRoot)
		}
	}
}

func TestAliDNSSignature(t *testing.T) {
	// Verify that doAPI produces a valid Signature parameter
	var captured url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.Query()
		w.Write([]byte(`{"RecordId":"123"}`))
	}))
	defer srv.Close()

	old := dnsHTTP
	dnsHTTP = srv.Client()
	defer func() { dnsHTTP = old }()

	p := &alidnsDNS{accessKey: "testAK", secretKey: "testSK"}
	// Temporarily override endpoint — call doAPI manually with test URL
	params := map[string]string{"Action": "AddDomainRecord", "DomainName": "example.com", "RR": "_acme-challenge", "Type": "TXT", "Value": "v", "TTL": "60"}
	_, err := alidnsDoAPIWithURL(p, srv.URL, params)
	if err != nil {
		t.Fatalf("doAPI: %v", err)
	}
	if captured.Get("Signature") == "" {
		t.Error("Signature missing")
	}
	if captured.Get("SignatureMethod") != "HMAC-SHA1" {
		t.Errorf("SignatureMethod = %q, want HMAC-SHA1", captured.Get("SignatureMethod"))
	}
	if captured.Get("AccessKeyId") != "testAK" {
		t.Errorf("AccessKeyId = %q, want testAK", captured.Get("AccessKeyId"))
	}
}

func TestAliDNSCleanUp(t *testing.T) {
	var actions []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		actions = append(actions, q.Get("Action"))
		switch q.Get("Action") {
		case "DescribeDomainRecords":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"DomainRecords": map[string]interface{}{
					"Record": []map[string]interface{}{
						{"RecordId": "456", "Value": "match"},
						{"RecordId": "789", "Value": "other"},
					},
				},
			})
		case "DeleteDomainRecord":
			if q.Get("RecordId") != "456" {
				t.Errorf("RecordId = %q, want 456", q.Get("RecordId"))
			}
			w.Write([]byte(`{"RecordId":"456"}`))
		}
	}))
	defer srv.Close()

	old := dnsHTTP
	dnsHTTP = srv.Client()
	defer func() { dnsHTTP = old }()

	p := &alidnsDNS{accessKey: "ak", secretKey: "sk"}
	err := alidnsCleanUpWithURL(p, srv.URL, "example.com", "tok", "match")
	if err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 API calls, got %d: %v", len(actions), actions)
	}
}
