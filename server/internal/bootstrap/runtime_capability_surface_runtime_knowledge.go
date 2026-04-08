package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerRuntimeTaskKnowledgeSurface(options runtimeTaskSurfaceOptions) runtimeTaskKnowledgeSurfaceRegistration {
	service := newRuntimeKnowledgeService(options)
	if service == nil {
		return runtimeTaskKnowledgeSurfaceRegistration{}
	}
	handler := knowledge.NewHandler(service)
	registered := registerKnowledgeRouteGroups(handler, runtimeTaskSurfaceProjectionGroups(options, "/knowledge", options.chatPermission))
	return runtimeTaskKnowledgeSurfaceRegistration{
		routesRegistered: registered,
		cronRegistered:   registerKnowledgeCronSupport(options, service),
	}
}

func registerKnowledgeRouteGroups(handler interface{ RegisterGroup(*echo.Group) }, groups []*echo.Group) bool {
	if handler == nil {
		return false
	}
	registered := false
	for _, group := range groups {
		if group == nil {
			continue
		}
		handler.RegisterGroup(group)
		registered = true
	}
	return registered
}

func newRuntimeKnowledgeService(options runtimeTaskSurfaceOptions) *knowledge.Service {
	workspaceDir := strings.TrimSpace(options.workspaceDir)
	if workspaceDir == "" {
		return nil
	}
	service := knowledge.NewService(knowledge.ServiceOptions{
		WorkspaceDir:          workspaceDir,
		RepoRoot:              resolveKnowledgeRepoRoot(workspaceDir),
		MemorySink:            runtimeKnowledgeMemorySink{handler: options.memoryHandler},
		KnowledgeAuthor:       options.knowledgeAuthor,
		DefaultLintProviderID: runtimeKnowledgeLintProviderSelector(options.settingsHandler),
		EventPublisher: runtimeKnowledgeEventPublisher{
			broker: options.sseBroker,
		},
		OnCompileSuccess: func(_ context.Context, _ *knowledge.KnowledgeCompileReport) {
			if options.cronHandler == nil {
				return
			}
			cronSvc := options.cronHandler.GetService()
			if cronSvc == nil {
				return
			}
			if err := ensureNightlyKnowledgeLintJob(cronSvc, options.logger); err != nil && options.logger != nil {
				options.logger.Warn("failed to ensure nightly knowledge lint cron job", zap.Error(err))
			}
		},
	})
	if options.chatHandler != nil {
		options.chatHandler.SetKnowledgeRetriever(service)
	}
	return service
}

func runtimeKnowledgeLintProviderSelector(settings *serverpkg.SettingsHandler) func() string {
	if settings == nil {
		return nil
	}
	return func() string {
		if !settings.GetSmallModelEnabled() || !settings.GetSmallModelKnowledgeFixEnabled() {
			return ""
		}
		return smallmodelProviderID
	}
}

type runtimeKnowledgeMemorySink struct {
	handler *serverpkg.MemoryHandler
}

func (s runtimeKnowledgeMemorySink) Remember(ctx context.Context, content string, tags []string) error {
	if s.handler == nil {
		return nil
	}
	s.handler.Init()
	service := s.handler.GetUnifiedService()
	if service == nil {
		return nil
	}
	_, err := service.Remember(ctx, content, tags)
	return err
}

type runtimeKnowledgeEventPublisher struct {
	broker interface {
		Publish(userID string, eventType string, data any)
	}
}

func (p runtimeKnowledgeEventPublisher) Publish(userID string, eventType string, data any) {
	if p.broker != nil {
		p.broker.Publish(userID, eventType, data)
	}
}

func resolveKnowledgeRepoRoot(workspaceDir string) string {
	candidates := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(workspaceDir); trimmed != "" {
		candidates = append(candidates, trimmed)
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		candidates = append(candidates, wd)
	}
	for _, candidate := range candidates {
		if root := findKnowledgeRepoRoot(candidate); root != "" {
			return root
		}
	}
	if len(candidates) > 0 {
		return filepath.Clean(candidates[0])
	}
	return ""
}

func findKnowledgeRepoRoot(start string) string {
	start = strings.TrimSpace(start)
	if start == "" {
		return ""
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		absStart = filepath.Clean(start)
	}
	current := absStart
	for {
		if knowledgeRepoRootLooksValid(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func knowledgeRepoRootLooksValid(root string) bool {
	if root == "" {
		return false
	}
	for _, marker := range []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "ARCHITECTURE.md"),
		filepath.Join(root, "docs"),
	} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}
