package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
)

type runtimeSchedulerSkillTarget interface {
	SetCronService(svc builtin.CronServiceInterface)
}

type runtimeSchedulerCalendarTarget interface {
	SetCronService(service tools.CronService)
}

type runtimeCronHandlerTarget interface {
	GetService() *cron.Service
	SetServiceInitHook(fn func(*cron.Service))
}

type runtimeReminderCalendarTarget interface {
	SetReminderService(service tools.PushServiceInterface)
}

type runtimeReminderSkillTarget interface {
	SetPushService(svc builtin.PushServiceInterface)
}

type runtimeSpeechServiceSource interface {
	Service() speech.Service
}

type runtimeVoiceServiceSource interface {
	Service() voice.Service
}
