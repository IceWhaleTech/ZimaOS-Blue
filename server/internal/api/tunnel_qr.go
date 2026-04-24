package api

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func buildTunnelQRURL(baseURL string, jwtService *auth.JWTService, userClaims *auth.UserClaims) string {
	qrURL := strings.TrimSpace(baseURL)
	if qrURL == "" {
		return ""
	}

	qrURL = strings.TrimRight(qrURL, "/") + "/chat"
	if jwtService != nil && userClaims != nil {
		token, err := jwtService.GenerateAccessToken(userClaims)
		if err == nil && strings.TrimSpace(token) != "" {
			qrURL += "?access_token=" + token
		}
	}

	return qrURL
}
