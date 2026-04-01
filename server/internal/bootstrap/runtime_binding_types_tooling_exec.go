package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeMgmtToolTarget interface {
	SetProviders(svc tools.AdminProviderService)
	SetSkills(svc tools.AdminSkillService)
	SetTools(svc tools.AdminToolService)
	SetSystem(svc tools.AdminSystemService)
	SetUsers(svc tools.AdminUserService)
	SetAPIKeys(svc tools.AdminAPIKeyService)
	SetUpgrade(svc tools.AdminUpgradeService)
}

type runtimeExecToolTarget interface {
	SetAuditStore(store *tools.ExecAuditStore)
	SetToolNames(names []string)
	SetRegistry(registry *tools.Registry)
	SetPinnedSkills(names []string)
	SetSkillExecutor(fn tools.SkillExecFunc)
	SetSkillSelector(fn tools.SkillSelectFunc)
}

type runtimeExecSkillSelectionSource interface {
	GetSkillSelector() *agentcore.SkillSelector
	GetSettingsHandler() *serverpkg.SettingsHandler
}
