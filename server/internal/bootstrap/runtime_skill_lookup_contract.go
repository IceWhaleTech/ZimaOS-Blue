package bootstrap

import skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"

type runtimeSkillRegistrySource interface {
	Get(id string) skillpkg.Skill
	IsEnabled(id string) bool
}
