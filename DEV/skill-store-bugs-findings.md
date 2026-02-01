# Skill Store Bug Fixes - Findings

## Created: 2026-02-02

## Key Files
- Frontend: `web/src/components/extensions/SkillStoreTab.vue`
- Frontend API: `web/src/api/skill.ts`
- Backend Handler: `server/internal/server/skill_handler.go`
- Backend Store: `server/internal/skillstore/store.go`
- Backend Sync: `server/internal/skillstore/sync.go`

## Discoveries

### Discovery 1: Category hardcoded to "skill"
In sync.go:458, category is hardcoded: `Category: "skill"` instead of using API data.
ClawHub API likely returns category info but it's not being mapped.

### Discovery 2: No click handler for skill homepage
In SkillStoreTab.vue, skill cards have no click handler to navigate to homepage.
RemoteSkill has `homepage` field but it's not used.

### Discovery 3: Search uses FTS5 with complex query
Search in store.go uses FTS5 full-text search. Need to check if FTS5 table is properly populated.

### Discovery 4: Pagination uses page/page_size, not cursor
Current pagination uses offset-based (page/page_size), not cursor-based.

## Root Causes

### BUG-1 Categories:
sync.go hardcodes `Category: "skill"` - need to map from ClawHub API category field

### BUG-2 Homepage Link:
SkillStoreTab.vue has no @click handler on skill cards to open homepage URL

### BUG-3 Search API:
Need to verify - likely FTS5 query escaping or table sync issue

### BUG-4 Infinite Loading:
Need to implement cursor-based pagination with cursor+count params
