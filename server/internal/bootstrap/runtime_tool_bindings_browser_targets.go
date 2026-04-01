package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeBrowserTargets(
	mediaDir string,
	browserBackend tools.BrowserBackend,
	lightpanda *browser.LightpandaService,
	browserTool runtimeBrowserMediaDirTarget,
	browserSkill runtimeBrowserSkillTarget,
	uiSkill runtimeUIReviewerSkillTarget,
	skillService builtin.BrowserServiceInterface,
	accessTargets ...runtimeBrowserAccessTarget,
) {
	if browserTool != nil {
		browserTool.SetMediaDir(mediaDir)
	}
	for _, target := range accessTargets {
		if target == nil {
			continue
		}
		if browserBackend != nil {
			target.SetBrowser(browserBackend)
		}
		if lightpanda != nil {
			target.SetLightpandaShim(lightpanda)
		}
	}
	if skillService != nil {
		if browserSkill != nil {
			browserSkill.SetBrowserService(skillService)
			browserSkill.SetMediaDir(mediaDir)
		}
		if uiSkill != nil {
			uiSkill.SetBrowserService(skillService)
		}
	}
}
