package bootstrap

import (
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (binding *runtimeContractBinding) BindProductivityTools(options routeRuntimeContractProductivityOptions) routeRuntimeContractProductivityResult {
	if binding == nil || options.writeDB == nil {
		return routeRuntimeContractProductivityResult{}
	}
	if options.readDB == nil {
		options.readDB = options.writeDB
	}
	if options.emailSkill == nil {
		options.emailSkill = runtimeSkillAsEmailTarget(options.skillRegistry, "email")
	}
	if options.calendarSkill == nil {
		options.calendarSkill = runtimeSkillAsCalendarTarget(options.skillRegistry, "calendar")
	}
	if options.contactsSkill == nil {
		options.contactsSkill = runtimeSkillAsContactsTarget(options.skillRegistry, "contacts")
	}

	result := routeRuntimeContractProductivityResult{}
	emailService, err := tools.NewLocalEmailServiceWithReadDB(options.writeDB, options.readDB)
	if err != nil {
		if options.logger != nil {
			options.logger.Warn("Failed to initialize local email tool", zap.Error(err))
		}
	} else {
		result.emailService = emailService
		result.emailTool = tools.RegisterEmailTool(options.registry, emailService)
		if options.emailSkill != nil && result.emailTool != nil {
			options.emailSkill.SetExecutor(result.emailTool)
		}
	}

	calendarService, err := tools.NewLocalCalendarServiceWithReadDB(options.writeDB, options.readDB)
	if err != nil {
		if options.logger != nil {
			options.logger.Warn("Failed to initialize local calendar tool", zap.Error(err))
		}
		return result
	}
	result.calendarService = calendarService
	result.calendarTool = tools.RegisterCalendarTool(options.registry, tools.PreferredCalendarStore(calendarService))
	if result.calendarTool != nil && result.emailService != nil {
		result.calendarTool.SetEmailService(result.emailService)
	}
	if options.calendarSkill != nil && result.calendarTool != nil {
		options.calendarSkill.SetExecutor(result.calendarTool)
	}

	contactsStore := tools.PreferredContactsStore(tools.NewUnavailableContactsStore(""))
	result.contactsStore = contactsStore
	result.contactsTool = tools.RegisterContactsTool(options.registry, contactsStore)
	if options.contactsSkill != nil && result.contactsTool != nil {
		options.contactsSkill.SetExecutor(result.contactsTool)
	}
	return result
}
