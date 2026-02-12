package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// DockerContainer represents a Docker container
type DockerContainer struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	State   string            `json:"state"`
	Ports   []string          `json:"ports,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Created string            `json:"created"`
}

// DockerConfig holds Docker configuration
type DockerConfig struct {
	Host       string `json:"host"`
	APIVersion string `json:"api_version"`
	TLSVerify  bool   `json:"tls_verify"`
	CertPath   string `json:"cert_path"`
}

// Docker is a built-in Docker management skill
type Docker struct {
	manifest *skill.Manifest
	config   *DockerConfig
	mu       sync.RWMutex
}

// NewDocker creates a new Docker skill
func NewDocker(config *DockerConfig) *Docker {
	return &Docker{
		manifest: &skill.Manifest{
			ID:          "docker",
			Name:        "Docker",
			Version:     "1.0.0",
			Description: "Docker container management",
			Category:    "system",
			Icon:        "docker",
			Tags:        []string{"docker", "containers", "devops"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: list, start, stop, restart, logs, inspect, images, pull",
					Required:    true,
				},
				{
					Name:        "container",
					Type:        "string",
					Description: "Container ID or name",
					Required:    false,
				},
				{
					Name:        "image",
					Type:        "string",
					Description: "Image name (for pull action)",
					Required:    false,
				},
				{
					Name:        "all",
					Type:        "boolean",
					Description: "Include stopped containers (for list)",
					Required:    false,
					Default:     false,
				},
				{
					Name:        "tail",
					Type:        "number",
					Description: "Number of log lines (for logs)",
					Required:    false,
					Default:     100,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "containers",
					Type:        "array",
					Description: "List of containers",
				},
				{
					Name:        "container",
					Type:        "object",
					Description: "Container details",
				},
				{
					Name:        "logs",
					Type:        "string",
					Description: "Container logs",
				},
			},
			Permissions: []string{"docker.read", "docker.write"},
		},
		config: config,
	}
}

// Manifest returns the skill manifest
func (d *Docker) Manifest() *skill.Manifest {
	return d.manifest
}

// Validate validates the input parameters
func (d *Docker) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"list": true, "start": true, "stop": true, "restart": true,
		"logs": true, "inspect": true, "images": true, "pull": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	containerActions := map[string]bool{
		"start": true, "stop": true, "restart": true, "logs": true, "inspect": true,
	}
	if containerActions[actionStr] {
		if _, ok := input["container"]; !ok {
			return fmt.Errorf("container is required for %s action", actionStr)
		}
	}

	if actionStr == "pull" {
		if _, ok := input["image"]; !ok {
			return fmt.Errorf("image is required for pull action")
		}
	}

	return nil
}

// Execute executes the Docker skill
func (d *Docker) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "list":
		return d.listContainers(ctx, input)
	case "start":
		return d.startContainer(ctx, input)
	case "stop":
		return d.stopContainer(ctx, input)
	case "restart":
		return d.restartContainer(ctx, input)
	case "logs":
		return d.getContainerLogs(ctx, input)
	case "inspect":
		return d.inspectContainer(ctx, input)
	case "images":
		return d.listImages(ctx)
	case "pull":
		return d.pullImage(ctx, input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (d *Docker) listContainers(ctx context.Context, input map[string]any) (*skill.Result, error) {
	// Placeholder: In production, use Docker SDK
	return skill.NewResult(map[string]any{
		"containers": []DockerContainer{},
		"message":    "Docker container listing placeholder. Configure Docker connection for actual data.",
	}), nil
}

func (d *Docker) startContainer(ctx context.Context, input map[string]any) (*skill.Result, error) {
	container := input["container"].(string)
	return skill.NewResult(map[string]any{
		"container": container,
		"started":   false,
		"message":   fmt.Sprintf("Docker start placeholder for container '%s'", container),
	}), nil
}

func (d *Docker) stopContainer(ctx context.Context, input map[string]any) (*skill.Result, error) {
	container := input["container"].(string)
	return skill.NewResult(map[string]any{
		"container": container,
		"stopped":   false,
		"message":   fmt.Sprintf("Docker stop placeholder for container '%s'", container),
	}), nil
}

func (d *Docker) restartContainer(ctx context.Context, input map[string]any) (*skill.Result, error) {
	container := input["container"].(string)
	return skill.NewResult(map[string]any{
		"container": container,
		"restarted": false,
		"message":   fmt.Sprintf("Docker restart placeholder for container '%s'", container),
	}), nil
}

func (d *Docker) getContainerLogs(ctx context.Context, input map[string]any) (*skill.Result, error) {
	container := input["container"].(string)
	tail := 100
	if t, ok := input["tail"].(float64); ok {
		tail = int(t)
	}

	return skill.NewResult(map[string]any{
		"container": container,
		"tail":      tail,
		"logs":      "",
		"message":   fmt.Sprintf("Docker logs placeholder for container '%s'", container),
	}), nil
}

func (d *Docker) inspectContainer(ctx context.Context, input map[string]any) (*skill.Result, error) {
	container := input["container"].(string)
	return skill.NewResult(map[string]any{
		"container": container,
		"details":   nil,
		"message":   fmt.Sprintf("Docker inspect placeholder for container '%s'", container),
	}), nil
}

func (d *Docker) listImages(ctx context.Context) (*skill.Result, error) {
	return skill.NewResult(map[string]any{
		"images":  []map[string]any{},
		"message": "Docker images listing placeholder",
	}), nil
}

func (d *Docker) pullImage(ctx context.Context, input map[string]any) (*skill.Result, error) {
	image := input["image"].(string)
	return skill.NewResult(map[string]any{
		"image":   image,
		"pulled":  false,
		"message": fmt.Sprintf("Docker pull placeholder for image '%s'", image),
	}), nil
}

// GitHub is a built-in GitHub operations skill
type GitHub struct {
	manifest *skill.Manifest
	config   *GitHubConfig
}

// GitHubConfig holds GitHub configuration
type GitHubConfig struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// NewGitHub creates a new GitHub skill
func NewGitHub(config *GitHubConfig) *GitHub {
	return &GitHub{
		manifest: &skill.Manifest{
			ID:          "github",
			Name:        "GitHub",
			Version:     "1.0.0",
			Description: "GitHub repository operations",
			Category:    "integration",
			Icon:        "github",
			Tags:        []string{"github", "git", "repository", "code"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: repos, issues, prs, search, user",
					Required:    true,
				},
				{
					Name:        "owner",
					Type:        "string",
					Description: "Repository owner",
					Required:    false,
				},
				{
					Name:        "repo",
					Type:        "string",
					Description: "Repository name",
					Required:    false,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query",
					Required:    false,
				},
				{
					Name:        "state",
					Type:        "string",
					Description: "Issue/PR state: open, closed, all",
					Required:    false,
					Default:     "open",
				},
				{
					Name:        "limit",
					Type:        "number",
					Description: "Number of results",
					Required:    false,
					Default:     10,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "repos",
					Type:        "array",
					Description: "List of repositories",
				},
				{
					Name:        "issues",
					Type:        "array",
					Description: "List of issues",
				},
				{
					Name:        "prs",
					Type:        "array",
					Description: "List of pull requests",
				},
			},
			Permissions: []string{"github.read", "github.write"},
		},
		config: config,
	}
}

// Manifest returns the skill manifest
func (g *GitHub) Manifest() *skill.Manifest {
	return g.manifest
}

// Validate validates the input parameters
func (g *GitHub) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"repos": true, "issues": true, "prs": true, "search": true, "user": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	repoActions := map[string]bool{"issues": true, "prs": true}
	if repoActions[actionStr] {
		if _, ok := input["owner"]; !ok {
			return fmt.Errorf("owner is required for %s action", actionStr)
		}
		if _, ok := input["repo"]; !ok {
			return fmt.Errorf("repo is required for %s action", actionStr)
		}
	}

	if actionStr == "search" {
		if _, ok := input["query"]; !ok {
			return fmt.Errorf("query is required for search action")
		}
	}

	return nil
}

// Execute executes the GitHub skill
func (g *GitHub) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "repos":
		return g.listRepos(ctx, input)
	case "issues":
		return g.listIssues(ctx, input)
	case "prs":
		return g.listPRs(ctx, input)
	case "search":
		return g.searchRepos(ctx, input)
	case "user":
		return g.getUserInfo(ctx)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (g *GitHub) listRepos(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if g.config == nil || g.config.Token == "" {
		return skill.NewResult(map[string]any{
			"repos":   []map[string]any{},
			"message": "GitHub not configured. Please configure token for actual data.",
		}), nil
	}

	return skill.NewResult(map[string]any{
		"repos":   []map[string]any{},
		"message": "GitHub repos placeholder",
	}), nil
}

func (g *GitHub) listIssues(ctx context.Context, input map[string]any) (*skill.Result, error) {
	owner := input["owner"].(string)
	repo := input["repo"].(string)
	state := "open"
	if s, ok := input["state"].(string); ok {
		state = s
	}

	return skill.NewResult(map[string]any{
		"owner":   owner,
		"repo":    repo,
		"state":   state,
		"issues":  []map[string]any{},
		"message": fmt.Sprintf("GitHub issues placeholder for %s/%s", owner, repo),
	}), nil
}

func (g *GitHub) listPRs(ctx context.Context, input map[string]any) (*skill.Result, error) {
	owner := input["owner"].(string)
	repo := input["repo"].(string)
	state := "open"
	if s, ok := input["state"].(string); ok {
		state = s
	}

	return skill.NewResult(map[string]any{
		"owner":   owner,
		"repo":    repo,
		"state":   state,
		"prs":     []map[string]any{},
		"message": fmt.Sprintf("GitHub PRs placeholder for %s/%s", owner, repo),
	}), nil
}

func (g *GitHub) searchRepos(ctx context.Context, input map[string]any) (*skill.Result, error) {
	query := input["query"].(string)

	return skill.NewResult(map[string]any{
		"query":   query,
		"repos":   []map[string]any{},
		"message": fmt.Sprintf("GitHub search placeholder for '%s'", query),
	}), nil
}

func (g *GitHub) getUserInfo(ctx context.Context) (*skill.Result, error) {
	if g.config == nil || g.config.Token == "" {
		return skill.NewResult(map[string]any{
			"user":    nil,
			"message": "GitHub not configured",
		}), nil
	}

	return skill.NewResult(map[string]any{
		"user":    nil,
		"message": "GitHub user info placeholder",
	}), nil
}

// Notion is a built-in Notion integration skill
type Notion struct {
	manifest *skill.Manifest
	config   *NotionConfig
}

// NotionConfig holds Notion configuration
type NotionConfig struct {
	Token string `json:"token"`
}

// NewNotion creates a new Notion skill
func NewNotion(config *NotionConfig) *Notion {
	return &Notion{
		manifest: &skill.Manifest{
			ID:          "notion",
			Name:        "Notion",
			Version:     "1.0.0",
			Description: "Notion workspace integration",
			Category:    "integration",
			Icon:        "notion",
			Tags:        []string{"notion", "notes", "workspace", "productivity"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: search, pages, databases, create",
					Required:    true,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query",
					Required:    false,
				},
				{
					Name:        "page_id",
					Type:        "string",
					Description: "Page ID",
					Required:    false,
				},
				{
					Name:        "database_id",
					Type:        "string",
					Description: "Database ID",
					Required:    false,
				},
				{
					Name:        "title",
					Type:        "string",
					Description: "Page title (for create)",
					Required:    false,
				},
				{
					Name:        "content",
					Type:        "string",
					Description: "Page content (for create)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "results",
					Type:        "array",
					Description: "Search results",
				},
				{
					Name:        "page",
					Type:        "object",
					Description: "Page details",
				},
			},
			Permissions: []string{"notion.read", "notion.write"},
		},
		config: config,
	}
}

// Manifest returns the skill manifest
func (n *Notion) Manifest() *skill.Manifest {
	return n.manifest
}

// Validate validates the input parameters
func (n *Notion) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"search": true, "pages": true, "databases": true, "create": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "create" {
		if _, ok := input["title"]; !ok {
			return fmt.Errorf("title is required for create action")
		}
	}

	return nil
}

// Execute executes the Notion skill
func (n *Notion) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	if n.config == nil || n.config.Token == "" {
		return skill.NewResult(map[string]any{
			"message": "Notion not configured. Please configure token for actual data.",
		}), nil
	}

	switch action {
	case "search":
		query := ""
		if q, ok := input["query"].(string); ok {
			query = q
		}
		return skill.NewResult(map[string]any{
			"query":   query,
			"results": []map[string]any{},
			"message": "Notion search placeholder",
		}), nil

	case "pages":
		return skill.NewResult(map[string]any{
			"pages":   []map[string]any{},
			"message": "Notion pages placeholder",
		}), nil

	case "databases":
		return skill.NewResult(map[string]any{
			"databases": []map[string]any{},
			"message":   "Notion databases placeholder",
		}), nil

	case "create":
		title := input["title"].(string)
		return skill.NewResult(map[string]any{
			"title":   title,
			"created": false,
			"message": fmt.Sprintf("Notion create placeholder for '%s'", title),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// SlackSkill is a built-in Slack operations skill
type SlackSkill struct {
	manifest *skill.Manifest
	config   *SlackSkillConfig
}

// SlackSkillConfig holds Slack configuration
type SlackSkillConfig struct {
	Token string `json:"token"`
}

// NewSlackSkill creates a new Slack skill
func NewSlackSkill(config *SlackSkillConfig) *SlackSkill {
	return &SlackSkill{
		manifest: &skill.Manifest{
			ID:          "slack-skill",
			Name:        "Slack",
			Version:     "1.0.0",
			Description: "Slack workspace operations",
			Category:    "integration",
			Icon:        "slack",
			Tags:        []string{"slack", "messaging", "team"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: send, channels, users, search",
					Required:    true,
				},
				{
					Name:        "channel",
					Type:        "string",
					Description: "Channel ID or name",
					Required:    false,
				},
				{
					Name:        "message",
					Type:        "string",
					Description: "Message to send",
					Required:    false,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "channels",
					Type:        "array",
					Description: "List of channels",
				},
				{
					Name:        "users",
					Type:        "array",
					Description: "List of users",
				},
				{
					Name:        "sent",
					Type:        "boolean",
					Description: "Whether message was sent",
				},
			},
			Permissions: []string{"slack.read", "slack.write"},
		},
		config: config,
	}
}

// Manifest returns the skill manifest
func (s *SlackSkill) Manifest() *skill.Manifest {
	return s.manifest
}

// Validate validates the input parameters
func (s *SlackSkill) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"send": true, "channels": true, "users": true, "search": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "send" {
		if _, ok := input["channel"]; !ok {
			return fmt.Errorf("channel is required for send action")
		}
		if _, ok := input["message"]; !ok {
			return fmt.Errorf("message is required for send action")
		}
	}

	return nil
}

// Execute executes the Slack skill
func (s *SlackSkill) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	if s.config == nil || s.config.Token == "" {
		return skill.NewResult(map[string]any{
			"message": "Slack not configured. Please configure token.",
		}), nil
	}

	switch action {
	case "send":
		channel := input["channel"].(string)
		message := input["message"].(string)
		return skill.NewResult(map[string]any{
			"channel": channel,
			"message": message,
			"sent":    false,
			"info":    "Slack send placeholder",
		}), nil

	case "channels":
		return skill.NewResult(map[string]any{
			"channels": []map[string]any{},
			"message":  "Slack channels placeholder",
		}), nil

	case "users":
		return skill.NewResult(map[string]any{
			"users":   []map[string]any{},
			"message": "Slack users placeholder",
		}), nil

	case "search":
		query := ""
		if q, ok := input["query"].(string); ok {
			query = q
		}
		return skill.NewResult(map[string]any{
			"query":   query,
			"results": []map[string]any{},
			"message": "Slack search placeholder",
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// DiscordSkill is a built-in Discord operations skill
type DiscordSkill struct {
	manifest *skill.Manifest
	config   *DiscordSkillConfig
}

// DiscordSkillConfig holds Discord configuration
type DiscordSkillConfig struct {
	Token string `json:"token"`
}

// NewDiscordSkill creates a new Discord skill
func NewDiscordSkill(config *DiscordSkillConfig) *DiscordSkill {
	return &DiscordSkill{
		manifest: &skill.Manifest{
			ID:          "discord-skill",
			Name:        "Discord",
			Version:     "1.0.0",
			Description: "Discord server operations",
			Category:    "integration",
			Icon:        "discord",
			Tags:        []string{"discord", "messaging", "community"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: send, channels, guilds, members",
					Required:    true,
				},
				{
					Name:        "channel_id",
					Type:        "string",
					Description: "Channel ID",
					Required:    false,
				},
				{
					Name:        "guild_id",
					Type:        "string",
					Description: "Guild/Server ID",
					Required:    false,
				},
				{
					Name:        "message",
					Type:        "string",
					Description: "Message to send",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "channels",
					Type:        "array",
					Description: "List of channels",
				},
				{
					Name:        "guilds",
					Type:        "array",
					Description: "List of guilds",
				},
				{
					Name:        "sent",
					Type:        "boolean",
					Description: "Whether message was sent",
				},
			},
			Permissions: []string{"discord.read", "discord.write"},
		},
		config: config,
	}
}

// Manifest returns the skill manifest
func (d *DiscordSkill) Manifest() *skill.Manifest {
	return d.manifest
}

// Validate validates the input parameters
func (d *DiscordSkill) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"send": true, "channels": true, "guilds": true, "members": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "send" {
		if _, ok := input["channel_id"]; !ok {
			return fmt.Errorf("channel_id is required for send action")
		}
		if _, ok := input["message"]; !ok {
			return fmt.Errorf("message is required for send action")
		}
	}

	if actionStr == "channels" || actionStr == "members" {
		if _, ok := input["guild_id"]; !ok {
			return fmt.Errorf("guild_id is required for %s action", actionStr)
		}
	}

	return nil
}

// Execute executes the Discord skill
func (d *DiscordSkill) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	if d.config == nil || d.config.Token == "" {
		return skill.NewResult(map[string]any{
			"message": "Discord not configured. Please configure token.",
		}), nil
	}

	switch action {
	case "send":
		channelID := input["channel_id"].(string)
		message := input["message"].(string)
		return skill.NewResult(map[string]any{
			"channel_id": channelID,
			"message":    message,
			"sent":       false,
			"info":       "Discord send placeholder",
		}), nil

	case "channels":
		guildID := input["guild_id"].(string)
		return skill.NewResult(map[string]any{
			"guild_id": guildID,
			"channels": []map[string]any{},
			"message":  "Discord channels placeholder",
		}), nil

	case "guilds":
		return skill.NewResult(map[string]any{
			"guilds":  []map[string]any{},
			"message": "Discord guilds placeholder",
		}), nil

	case "members":
		guildID := input["guild_id"].(string)
		return skill.NewResult(map[string]any{
			"guild_id": guildID,
			"members":  []map[string]any{},
			"message":  "Discord members placeholder",
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// Helper function to check if string contains substring (case-insensitive)
func containsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
