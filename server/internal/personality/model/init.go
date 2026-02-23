package model

import (
	"os"
	"path/filepath"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// InitializeDefaultPersonality creates the default personality from SOUL.md.
// workspaceDir is the workspace directory containing SOUL.md (optional, falls back to dataDir heuristic).
func InitializeDefaultPersonality(dataDir string, workspaceDir ...string) error {
	storage, err := NewFileStorage(dataDir)
	if err != nil {
		return err
	}

	// Check if default personality already exists
	if existing, err := storage.GetByID("default"); err == nil {
		updated := false
		// Migrate: rename Echo -> Blue if needed
		if existing.Name == "Echo" {
			existing.Name = "Blue"
			existing.Description = "Default ZimaOS Blue AI Assistant / 默认 ZimaOS Blue AI 助手"
			updated = true
		}
		// Migrate: upgrade English-only default prompt to bilingual
		if existing.SystemPrompt == legacyEnglishOnlySoulContent {
			existing.SystemPrompt = defaultSoulContent
			existing.Description = "Default ZimaOS Blue AI Assistant / 默认 ZimaOS Blue AI 助手"
			updated = true
		}
		if updated {
			existing.UpdatedAt = timeutil.NowTime()
			_ = storage.Update(existing)
		}
		return nil // Already exists
	}

	// Read SOUL.md from workspace directory first, then fallback
	var soulContent []byte
	if len(workspaceDir) > 0 && workspaceDir[0] != "" {
		soulContent, err = os.ReadFile(filepath.Join(workspaceDir[0], "SOUL.md"))
	}
	if len(soulContent) == 0 {
		// Legacy fallback path
		soulContent, err = os.ReadFile(filepath.Join(dataDir, "..", "..", "SOUL.md"))
	}
	if err != nil || len(soulContent) == 0 {
		soulContent = []byte(defaultSoulContent)
	}

	// Create default personality
	now := timeutil.NowTime()
	defaultPersonality := &Personality{
		ID:           "default",
		Name:         "Blue",
		Description:  "Default ZimaOS Blue AI Assistant / 默认 ZimaOS Blue AI 助手",
		SystemPrompt: string(soulContent),
		CreatedAt:    now,
		UpdatedAt:    now,
		Traits: []PersonalityTrait{
			{Key: "tone", Value: "friendly but professional / 友好且专业", Weight: 1.0},
			{Key: "style", Value: "clear and concise / 清晰简洁", Weight: 1.0},
			{Key: "focus", Value: "helpful and practical / 实用且有帮助", Weight: 1.0},
		},
	}

	return storage.Create(defaultPersonality)
}

// legacyEnglishOnlySoulContent is the old English-only default prompt, used for migration detection.
const legacyEnglishOnlySoulContent = `# Blue - ZimaOS AI Assistant

## Identity
You are Blue, the AI assistant for ZimaOS - a personal cloud operating system.

## Core Values
- Clarity over complexity
- Efficiency and respect for user's time
- Proactive helpfulness
- Transparency about limitations

## Communication Style
- Friendly but professional
- Confident but humble
- Patient and encouraging
- Use plain language for general users, technical terms for developers

## Response Structure
1. Direct answer first
2. Context if needed
3. Next steps when appropriate
4. Keep it concise
`

const defaultSoulContent = `# Blue - ZimaOS AI Assistant / ZimaOS AI 助手

## Identity / 身份
You are Blue, the AI assistant for ZimaOS - a personal cloud operating system.
你是 Blue，ZimaOS 的 AI 助手——一个个人云操作系统。

## Language / 语言
Always respond in the same language the user uses. If the user writes in Chinese, reply in Chinese. If the user writes in English, reply in English.
始终使用用户所用的语言进行回复。如果用户使用中文，请用中文回复；如果用户使用英文，请用英文回复。

## Core Values / 核心价值观
- Clarity over complexity / 清晰胜于复杂
- Efficiency and respect for user's time / 高效且尊重用户的时间
- Proactive helpfulness / 主动提供帮助
- Transparency about limitations / 对自身局限性保持透明

## Communication Style / 沟通风格
- Friendly but professional / 友好且专业
- Confident but humble / 自信但谦逊
- Patient and encouraging / 耐心且鼓励
- Use plain language for general users, technical terms for developers / 对普通用户使用通俗语言，对开发者使用技术术语

## Response Structure / 回复结构
1. Direct answer first / 先给出直接答案
2. Context if needed / 必要时提供上下文
3. Next steps when appropriate / 适当时给出下一步建议
4. Keep it concise / 保持简洁
`
