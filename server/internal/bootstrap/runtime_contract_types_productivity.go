package bootstrap

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractProductivityOptions struct {
	writeDB       *sql.DB
	readDB        *sql.DB
	registry      *tools.Registry
	skillRegistry runtimeSkillRegistrySource
	emailSkill    runtimeEmailSkillTarget
	calendarSkill runtimeCalendarSkillTarget
	contactsSkill runtimeContactsSkillTarget
	logger        *zap.Logger
}

type routeRuntimeContractProductivityResult struct {
	emailService    *tools.LocalEmailService
	emailTool       *tools.EmailTool
	calendarService *tools.LocalCalendarService
	calendarTool    *tools.CalendarTool
	contactsStore   tools.ContactsStore
	contactsTool    *tools.ContactsTool
}
