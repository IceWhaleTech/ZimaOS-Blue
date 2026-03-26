package tools

var (
	webFetchBrowserPreferredHosts = []string{
		"github.com",
	}
	webFetchLightpandaUnsupportedHosts = []string{
		"github.com",
	}
)

func webFetchPrefersBrowserHost(host string) bool {
	return webFetchHostListContains(webFetchBrowserPreferredHosts, host)
}

func webFetchSupportsLightpandaHost(host string) bool {
	return !webFetchHostListContains(webFetchLightpandaUnsupportedHosts, host)
}
