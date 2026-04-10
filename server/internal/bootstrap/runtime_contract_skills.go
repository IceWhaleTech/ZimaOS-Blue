package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"

func skillRegistryFromRuntimeSource(source runtimeSkillRegistrySource) *skill.Registry {
	if source == nil {
		return nil
	}
	registry, _ := source.(*skill.Registry)
	return registry
}

func runtimeSkillAsAnalyzeTarget(source runtimeSkillRegistrySource, id string) runtimeAnalyzeSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeAnalyzeSkillTarget)
	return target
}

func runtimeSkillAsEmailTarget(source runtimeSkillRegistrySource, id string) runtimeEmailSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeEmailSkillTarget)
	return target
}

func runtimeSkillAsCalendarTarget(source runtimeSkillRegistrySource, id string) runtimeCalendarSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeCalendarSkillTarget)
	return target
}

func runtimeSkillAsContactsTarget(source runtimeSkillRegistrySource, id string) runtimeContactsSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeContactsSkillTarget)
	return target
}

func runtimeSkillAsWebSearchTarget(source runtimeSkillRegistrySource, id string) runtimeWebSearchSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeWebSearchSkillTarget)
	return target
}

func runtimeSkillAsDeepResearchTarget(source runtimeSkillRegistrySource, id string) runtimeDeepResearchSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeDeepResearchSkillTarget)
	return target
}

func runtimeSkillAsSchedulerTarget(source runtimeSkillRegistrySource, id string) runtimeSchedulerSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeSchedulerSkillTarget)
	return target
}

func runtimeSkillAsBrowserTarget(source runtimeSkillRegistrySource, id string) runtimeBrowserSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeBrowserSkillTarget)
	return target
}

func runtimeSkillAsUIReviewerTarget(source runtimeSkillRegistrySource, id string) runtimeUIReviewerSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeUIReviewerSkillTarget)
	return target
}

func runtimeSkillAsReminderTarget(source runtimeSkillRegistrySource, id string) runtimeReminderSkillTarget {
	skill := runtimeSkillFromSource(source, id)
	if skill == nil {
		return nil
	}
	target, _ := skill.(runtimeReminderSkillTarget)
	return target
}

func runtimeSkillFromSource(source runtimeSkillRegistrySource, id string) skill.Skill {
	if source == nil {
		return nil
	}
	return source.Get(id)
}
