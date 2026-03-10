package upstreamerrors

import (
	"bytes"
	"strings"
)

var requestBuildFailureMarkers = [][]byte{
	[]byte("构建请求失败"),
	[]byte("请求构建失败"),
	[]byte("failed to build request"),
	[]byte("build request failed"),
	[]byte("request build failed"),
	[]byte("failed building request"),
}

// HasRequestBuildFailureBody reports whether an upstream payload looks like a
// relay/request-construction failure rather than true provider throttling.
func HasRequestBuildFailureBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	lower := bytes.ToLower(body)
	for _, marker := range requestBuildFailureMarkers {
		if bytes.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// HasRequestBuildFailureText is the string variant of HasRequestBuildFailureBody.
func HasRequestBuildFailureText(msg string) bool {
	if strings.TrimSpace(msg) == "" {
		return false
	}
	return HasRequestBuildFailureBody([]byte(msg))
}
