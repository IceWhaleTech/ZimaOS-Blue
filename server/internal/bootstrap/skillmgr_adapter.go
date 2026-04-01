package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

// skillManagerAdapter implements sockipc.SkillManagerBackend by wrapping
// the existing skillstore.Store, skill.Registry, and local scanner.
type skillManagerAdapter struct {
	store    *skillstore.Store
	registry *skill.Registry
	scanner  *skillstore.LocalSkillScanner
	dir      string
	client   *http.Client
}

func newSkillManagerAdapter(
	store *skillstore.Store,
	registry *skill.Registry,
	scanner *skillstore.LocalSkillScanner,
	skillsDir string,
) sockipc.SkillManagerBackend {
	return &skillManagerAdapter{
		store:    store,
		registry: registry,
		scanner:  scanner,
		dir:      skillsDir,
		client:   network.NewPooledHTTPClient(5 * time.Minute),
	}
}

func (a *skillManagerAdapter) Search(ctx context.Context, query, category string, page, pageSize int) (string, error) {
	if a.store == nil {
		return "[]", nil
	}
	opts := skillstore.SearchOptions{
		Query:    query,
		Page:     page,
		PageSize: pageSize,
	}
	if category != "" {
		opts.Categories = []string{category}
	}
	result, err := a.store.Search(ctx, opts)
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func (a *skillManagerAdapter) Enable(ctx context.Context, id string) error {
	canonicalID, err := ensureRuntimeManagedSkillRegisteredWithRoots(a.registry, a.skillRoots(), id)
	if err != nil {
		return err
	}
	if err := a.registry.Enable(canonicalID); err != nil {
		return err
	}
	if a.store != nil {
		a.store.SetEnabled(ctx, canonicalID, true)
	}
	return nil
}

func (a *skillManagerAdapter) Disable(ctx context.Context, id string) error {
	canonicalID, err := ensureRuntimeManagedSkillRegisteredWithRoots(a.registry, a.skillRoots(), id)
	if err != nil {
		return err
	}
	if err := a.registry.Disable(canonicalID); err != nil {
		return err
	}
	if a.store != nil {
		a.store.SetEnabled(ctx, canonicalID, false)
	}
	return nil
}

func (a *skillManagerAdapter) skillRoots() []string {
	if strings.TrimSpace(a.dir) == "" {
		return nil
	}
	return skillmanifest.ResolvePeerRootsForManagedDir(a.dir)
}

func runtimeManagedSkillEnabled(registry *skill.Registry, doc skillmanifest.Document) bool {
	if registry != nil {
		if info := registry.GetInfo(strings.TrimSpace(doc.ID)); info != nil {
			return info.Enabled
		}
	}
	return doc.Enabled
}
