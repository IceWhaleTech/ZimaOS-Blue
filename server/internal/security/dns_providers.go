package security

import (
	"fmt"
	"strings"

	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/godaddy"
	"github.com/go-acme/lego/v4/providers/dns/namecheap"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
)

// dnsProvider is the interface that lego DNS providers implement.
type dnsProvider interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}

// newDNSProvider creates a DNS provider by name.
// Separated into its own file so the heavy imports (aws-sdk, etc.)
// are only linked if this function is actually referenced.
func newDNSProvider(name string) (dnsProvider, error) {
	switch strings.ToLower(name) {
	case "cloudflare":
		return cloudflare.NewDNSProvider()
	case "route53":
		return route53.NewDNSProvider()
	case "godaddy":
		return godaddy.NewDNSProvider()
	case "namecheap":
		return namecheap.NewDNSProvider()
	case "alidns", "aliyun":
		return alidns.NewDNSProvider()
	case "tencentcloud", "dnspod":
		return tencentcloud.NewDNSProvider()
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", name)
	}
}
