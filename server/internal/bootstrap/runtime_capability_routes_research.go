package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
)

type runtimeTaskResearchSurfaceOptions struct {
	research              *deepresearch.Service
	harness               *HarnessRuntimeBundle
	workspaceDir          string
	deepResearchGroups    []*echo.Group
	harnessResearchGroups []*echo.Group
}

type runtimeTaskResearchSurfaceRegistration struct {
	creatorBound              bool
	deepResearchRegistered    bool
	harnessResearchRegistered bool
}
