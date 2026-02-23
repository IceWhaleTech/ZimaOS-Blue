// Package builtin provides built-in skills for the skill hub.
package builtin

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// Config holds configuration for built-in skills
type Config struct {
	Files    *FilesConfig
	Email    *EmailConfig
	Calendar *CalendarConfig
	Contacts *ContactsConfig
	Docker   *DockerConfig
	GitHub   *GitHubConfig
	Notion   *NotionConfig
	Slack    *SlackSkillConfig
	Discord  *DiscordSkillConfig
}

// FilesConfig holds configuration for the files skill
type FilesConfig struct {
	BaseDir string
}

// RegisterAll registers all built-in skills with the registry
func RegisterAll(registry *skill.Registry) error {
	return RegisterAllWithConfig(registry, nil)
}

// RegisterAllWithConfig registers all built-in skills with optional configuration
func RegisterAllWithConfig(registry *skill.Registry, config *Config) error {
	var filesBaseDir string
	var emailConfig *EmailConfig
	var calendarConfig *CalendarConfig
	var contactsConfig *ContactsConfig
	var dockerConfig *DockerConfig
	var githubConfig *GitHubConfig
	var notionConfig *NotionConfig
	var slackConfig *SlackSkillConfig
	var discordConfig *DiscordSkillConfig

	if config != nil {
		emailConfig = config.Email
		calendarConfig = config.Calendar
		contactsConfig = config.Contacts
		dockerConfig = config.Docker
		githubConfig = config.GitHub
		notionConfig = config.Notion
		slackConfig = config.Slack
		discordConfig = config.Discord
		if config.Files != nil {
			filesBaseDir = config.Files.BaseDir
		}
	}

	skills := []skill.Skill{
		// Core skills (4)
		NewCalculator(),
		NewSystemInfo(),
		NewDateTime(),
		NewSearch(),

		// Productivity skills (4)
		NewTimer(),
		NewReminders(),
		NewNotes(),
		NewTasks(),

		// Utility skills (3)
		NewTranslate(),
		NewNotifications(),
		NewUnitConverter(),

		// System skills (9)
		NewFiles(filesBaseDir),
		NewNetwork(),
		NewProcesses(),
		NewDocker(dockerConfig),
		NewScheduler(),
		NewWorkflows(),
		NewAutoReply(),
		NewSandbox(),
		NewBrowser(),

		// Communication skills (3)
		NewEmail(emailConfig),
		NewCalendar(calendarConfig),
		NewContacts(contactsConfig),

		// Integration skills (4)
		NewGitHub(githubConfig),
		NewNotion(notionConfig),
		NewSlackSkill(slackConfig),
		NewDiscordSkill(discordConfig),
	}

	for _, s := range skills {
		if err := registry.Register(s, true); err != nil {
			return err
		}
	}

	return nil
}

// GetSkillCount returns the number of built-in skills
func GetSkillCount() int {
	return 27 // 4 + 4 + 3 + 9 + 3 + 4
}

