package api

import blueServer "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"

func resolveListeningPort(configuredPort int) int {
	if actualPort := blueServer.GetActualPort(); actualPort > 0 {
		return actualPort
	}
	if configuredPort > 0 {
		return configuredPort
	}
	return 0
}
