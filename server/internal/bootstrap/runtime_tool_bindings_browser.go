package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeBrowserSkillService(
	acquire func() (*browser.RodService, func(), error),
	lazy func() *browser.RodService,
) builtin.BrowserServiceInterface {
	switch {
	case acquire != nil:
		return newLeaseAwareBrowserSkillAdapter(acquire)
	case lazy != nil:
		return newLazyBrowserSkillAdapter(lazy)
	default:
		return nil
	}
}

func newRuntimeUIReviewerTool(
	acquire func() (*browser.RodService, func(), error),
	lazy func() *browser.RodService,
	mediaDir string,
) *tools.UIReviewerTool {
	uiTool := &tools.UIReviewerTool{}
	switch {
	case acquire != nil:
		uiTool.SetBrowser(tools.NewLeaseAwareRodBrowserAdapter(acquire))
	case lazy != nil:
		uiTool.SetBrowser(tools.NewLazyRodBrowserAdapter(lazy))
	default:
		return nil
	}
	uiTool.SetMediaDir(mediaDir)
	return uiTool
}
