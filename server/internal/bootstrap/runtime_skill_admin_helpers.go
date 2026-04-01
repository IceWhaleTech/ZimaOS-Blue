package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func adminSkillInfoFromDocument(doc skillmanifest.Document, enabled, builtin bool) tools.AdminSkillInfo {
	id := strings.TrimSpace(firstRuntimeSkillValue(doc.ID, doc.Name))
	name := strings.TrimSpace(firstRuntimeSkillValue(doc.Name, doc.ID))
	return tools.AdminSkillInfo{
		ID:             id,
		Name:           name,
		Category:       strings.TrimSpace(doc.Category),
		Enabled:        enabled,
		Builtin:        builtin,
		Paths:          append([]string(nil), doc.Paths...),
		UserInvocable:  doc.UserInvocable,
		ModelInvocable: doc.ModelInvocable,
	}
}

func firstRuntimeSkillValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
