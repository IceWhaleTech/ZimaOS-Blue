package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

func registerReflectTaskSurface(service *selfreflect.Service, groups []*echo.Group) bool {
	if service == nil {
		return false
	}
	handler := selfreflect.NewHandler(service)
	registered := false
	for _, group := range groups {
		if group == nil {
			continue
		}
		handler.RegisterRoutes(group)
		registered = true
	}
	return registered
}
