package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeReminderServices(
	registry *tools.Registry,
	pushService *push.Service,
	calendar runtimeReminderCalendarTarget,
	reminder runtimeReminderSkillTarget,
) {
	if pushService == nil {
		return
	}
	pushTools := push.NewToolsAdapter(func() *push.Service { return pushService })
	if registry != nil {
		tools.RegisterPushTool(registry, pushTools)
		tools.RegisterMessageTool(registry, pushTools)
	}
	if calendar != nil {
		calendar.SetReminderService(pushTools)
	}
	if reminder != nil {
		reminder.SetPushService(push.NewSkillAdapter(func() *push.Service { return pushService }))
	}
}
