package bootstrap

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func wireAskSupport(toolRegistry *tools.Registry, skillRegistry *skill.Registry, broker *sse.Broker, timeout time.Duration) *tools.QuestionManager {
	if broker == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	questionMgr := tools.NewQuestionManager(broker, nil, timeout)
	if toolRegistry != nil {
		tools.RegisterAskTool(toolRegistry, questionMgr)
	}
	if skillRegistry != nil {
		if sk := skillRegistry.Get("ask"); sk != nil {
			if askSkill, ok := sk.(*builtin.Ask); ok {
				askSkill.SetQuestioner(&questionManagerAskAdapter{mgr: questionMgr})
			}
		}
	}
	return questionMgr
}
