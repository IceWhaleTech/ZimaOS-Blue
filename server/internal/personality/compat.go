package personality

import (
	"database/sql"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/personality/controller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/personality/model"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/personality/view"
)

// 向后兼容性导出
type (
	Personality      = model.Personality
	PersonalityTrait = model.PersonalityTrait
	Repository       = model.Repository
	Service          = controller.Service
	Handler          = view.Handler
)

// 向后兼容性函数
func NewPersonality(name, description, systemPrompt string) *Personality {
	return model.NewPersonality(name, description, systemPrompt)
}

func NewRepository(db *sql.DB) *Repository {
	return model.NewRepository(db)
}

func NewService(repo *Repository) *Service {
	return controller.NewService(repo)
}

func NewHandler(service *Service) *Handler {
	return view.NewHandler(service)
}

