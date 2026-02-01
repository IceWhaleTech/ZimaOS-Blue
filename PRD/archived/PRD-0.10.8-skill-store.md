# v0.10.8 Skill Store Improvements PRD

## Overview

Version 0.10.8 focuses on improving the skill store experience by addressing fetch failures, enhancing search/install/uninstall UX, and ensuring skills are properly visible to Claude Code CLI.

## Problem Statement

### Current Issues

1. **Skill Store Fetch Failures**
   - ClawHub API unreliable (network timeouts, rate limits)
   - No graceful degradation when external sources fail
   - Users see empty store or error messages

2. **Poor Search Experience**
   - No multi-language search support
   - Search only works on exact matches
   - No fuzzy matching or relevance scoring in UI

3. **Install/Uninstall UX Issues**
   - No progress indication during install
   - No confirmation before uninstall
   - No rollback on failed installs
   - Unclear error messages

4. **Skill Visibility Verification**
   - No way to verify if installed skills are visible to CC CLI
   - No test suite to confirm skill registration works

## Solution Design

### Architecture Changes

#### 1. Hybrid Approach: External API + Local Fallback

Keep ClawHub/GitHub API support while adding reliability improvements:

- **Primary**: ClawHub/GitHub API for skill discovery
- **Fallback**: Featured Skills list when API fails
- **Local**: Support local skill installation from `~/.claude/skills/`
- **Caching**: Cache API results locally for offline access
- **No total count**: Don't display total count (unreliable), show only installed count

#### 2. New Skill Source Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Skill Sources                            │
├─────────────────────────────────────────────────────────────┤
│  1. Built-in Skills (bundled with app)                      │
│  2. ClawHub API (primary external source)                   │
│  3. GitHub API (secondary external source)                  │
│  4. Featured Skills (fallback when API fails)               │
│  5. Local Skills (from ~/.claude/skills/ directory)         │
│  6. Custom Skills (user-added via URL)                      │
└─────────────────────────────────────────────────────────────┘
```

#### 3. Fallback Strategy

```
API Request Flow:
1. Try ClawHub/GitHub API
2. If success → cache results locally, return data
3. If failure →
   a. Return cached data if available and fresh (< 24h)
   b. Return Featured Skills as fallback
   c. Show user-friendly error message
```

### Feature Specifications

#### F1: Featured Skills System

**Description**: Replace unreliable external store with curated featured skills list.

**Requirements**:
- Maintain a JSON file with featured skills metadata
- Cache featured skills locally for offline access
- Update featured list via app updates (not runtime API calls)
- Show "Featured" badge on curated skills

**Data Structure**:
```typescript
interface FeaturedSkill {
  id: string
  name: string
  description: string
  author: string
  category: string
  tags: string[]
  version: string
  source_url: string  // GitHub raw URL or direct download
  homepage: string
  stars?: number      // Static, updated with app releases
  featured_rank: number
}
```

#### F2: Local Skill Discovery

**Description**: Automatically discover skills from standard directories.

**Requirements**:
- Scan `~/.claude/skills/` directory for SKILL.md files
- Support vercel-labs/skills format (YAML frontmatter + markdown)
- Auto-register discovered skills on startup
- Watch directory for changes (hot reload)

**SKILL.md Format** (vercel-labs/skills compatible):
```markdown
---
name: my-skill
description: A helpful skill for X
version: 1.0.0
author: username
category: productivity
tags: [automation, helper]
---

# My Skill Instructions

Instructions for the AI agent...
```

#### F3: Improved Search

**Description**: Better search with multi-language support.

**Requirements**:
- Search across name, description, tags in all languages
- Normalize search queries (lowercase, trim, remove accents)
- Support partial matching (prefix search)
- Highlight matching terms in results
- Search installed skills and featured skills separately

**Search Algorithm**:
```
1. Normalize query: lowercase, remove accents, trim
2. Tokenize query into words
3. For each skill:
   - Score = sum of matches in (name * 3 + description * 2 + tags * 1)
   - Boost if exact match
   - Boost if prefix match
4. Sort by score descending
```

#### F4: Install/Uninstall UX Improvements

**Description**: Better feedback during skill operations.

**Requirements**:
- Show progress indicator during install
- Confirm before uninstall with skill name
- Show success/error toast notifications
- Retry failed installs automatically (up to 3 times)
- Rollback partial installs on failure

**Install Flow**:
```
1. User clicks Install
2. Show "Installing..." with spinner
3. Download skill manifest
4. Validate skill format
5. Register skill in registry
6. Update database
7. Show success toast
8. If any step fails: show error, cleanup partial state
```

#### F5: Skill Visibility Verification

**Description**: Test suite to verify skills are visible to CC CLI.

**Requirements**:
- Integration tests that verify skill registration
- Tests that simulate CC CLI skill discovery
- Verify skill can be invoked after install
- Verify skill is removed after uninstall

### API Changes

#### New Endpoints

```
GET  /skills/verify/:id          - Verify skill is registered and callable
GET  /skills/local               - List locally discovered skills
POST /skills/local/scan          - Trigger local skill directory scan
GET  /skill-store/featured       - Get featured skills list
POST /skill-store/install-url    - Install skill from URL
```

#### Modified Endpoints

```
GET  /skill-store/browse         - Returns featured + installed only (no external API)
POST /skill-store/refresh        - Refreshes local cache only (no external API)
```

### Database Changes

#### New Table: featured_skills

```sql
CREATE TABLE featured_skills (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  author TEXT,
  category TEXT,
  tags TEXT,
  version TEXT,
  source_url TEXT,
  homepage TEXT,
  stars INTEGER DEFAULT 0,
  featured_rank INTEGER DEFAULT 0,
  cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### New Table: local_skills

```sql
CREATE TABLE local_skills (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  file_path TEXT NOT NULL,
  version TEXT,
  author TEXT,
  category TEXT,
  tags TEXT,
  discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  last_modified TIMESTAMP
);
```

### UI Changes

#### Skill Store View

1. **Remove "Store" tab total count** - Show only installed count
2. **Rename tabs**:
   - "Installed (N)" - User's installed skills
   - "Featured" - Curated skill recommendations
   - "Local" - Skills from local directory
3. **Add "Install from URL" button**
4. **Add confirmation dialog for uninstall**
5. **Add progress indicator for install**
6. **Add toast notifications for operations**

#### Search Improvements

1. **Debounce search input** (300ms)
2. **Show "No results" with suggestions**
3. **Highlight matching text in results**

### Test Plan

#### Unit Tests

```go
// skill_handler_test.go additions

func TestSkillHandler_VerifySkill(t *testing.T)
func TestSkillHandler_ListLocalSkills(t *testing.T)
func TestSkillHandler_ScanLocalSkills(t *testing.T)
func TestSkillHandler_GetFeaturedSkills(t *testing.T)
func TestSkillHandler_InstallFromURL(t *testing.T)
func TestSkillHandler_InstallWithRetry(t *testing.T)
func TestSkillHandler_UninstallWithCleanup(t *testing.T)
```

#### Integration Tests

```go
// skill_integration_test.go (new file)

func TestSkillVisibility_AfterInstall(t *testing.T)
func TestSkillVisibility_AfterUninstall(t *testing.T)
func TestSkillVisibility_LocalDiscovery(t *testing.T)
func TestSkillVisibility_CCCLICompatibility(t *testing.T)
func TestSkillSearch_MultiLanguage(t *testing.T)
func TestSkillSearch_FuzzyMatch(t *testing.T)
```

#### E2E Tests

```typescript
// skill-store.spec.ts (Playwright)

test('install featured skill and verify visibility')
test('uninstall skill with confirmation')
test('search skills in multiple languages')
test('install skill from URL')
test('local skill discovery')
```

### Skill Visibility Verification Test Cases

These tests ensure installed skills are properly visible to Claude Code CLI:

```go
// TestSkillVisibility_CCCLIFormat verifies skill output matches CC CLI expectations
func TestSkillVisibility_CCCLIFormat(t *testing.T) {
    // 1. Install a skill
    // 2. Call GET /skills
    // 3. Verify response includes:
    //    - id (matches CC CLI skill ID format)
    //    - name (human readable)
    //    - description (for CC CLI to display)
    //    - enabled (CC CLI respects this)
    // 4. Verify skill can be invoked via POST /skills/:id/execute
}

// TestSkillVisibility_RegistrySync verifies skill registry is in sync
func TestSkillVisibility_RegistrySync(t *testing.T) {
    // 1. Install skill via API
    // 2. Verify skill appears in registry.List()
    // 3. Verify skill.Manifest() returns correct data
    // 4. Uninstall skill
    // 5. Verify skill removed from registry
}

// TestSkillVisibility_PersistenceAcrossRestart verifies skills survive restart
func TestSkillVisibility_PersistenceAcrossRestart(t *testing.T) {
    // 1. Install skill
    // 2. Simulate server restart (reinitialize registry from DB)
    // 3. Verify skill still visible
    // 4. Verify skill still executable
}
```

### Migration Plan

1. **Phase 1**: Add featured skills system (no breaking changes)
2. **Phase 2**: Add local skill discovery
3. **Phase 3**: Deprecate external store API calls
4. **Phase 4**: Remove external store code

### Success Metrics

1. **Reliability**: 0 fetch failures (no external API dependency)
2. **Performance**: Skill list loads in < 100ms
3. **UX**: Install/uninstall operations complete with clear feedback
4. **Compatibility**: 100% of installed skills visible to CC CLI

### Out of Scope

- Skill marketplace with user submissions
- Skill ratings/reviews
- Skill auto-updates
- Skill dependencies resolution

## Implementation Checklist

See [v0.10.8-checklist.md](./v0.10.8-checklist.md) for detailed implementation tasks.

## References

- [vercel-labs/skills](https://github.com/vercel-labs/skills) - Agent Skills specification
- [skills.sh](https://skills.sh) - Skills discovery website
- Current implementation: [skill_handler.go](../server/internal/server/skill_handler.go)
