package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

var (
	skillsEligible          bool
	skillsInstalled         bool
	skillsSearchSort        string
	skillsSearchCat         string
	skillsSearchSource      string
	skillsSearchRisk        string
	skillsSearchInstallType string
	skillsSearchArtifact    string
	skillsSearchSem         bool
	skillsSearchSafeOnly    bool
	skillsSearchCurated     bool
	skillsTrendingCat       string
	skillsTrendingSource    string
	skillsTrendingCurated   bool
	skillsTrendingLim       int
	skillsUpdateAll         bool
	skillsAckRisk           bool
	skillsExit              = os.Exit
)

// skillsCmd represents the skills command
var skillsCmd = &cobra.Command{
	Use:     "skills",
	Aliases: []string{"skill"},
	Short:   "Skill management",
	Long: `Manage ZimaOS-Blue skills.

Subcommands:
  list                    List local or installed skills
  search <query>          Search the skills marketplace
  trending                Show trending skills
  info <id>               Show skill details
  security <id>           Show the security report
  install <id|github:*>   Install a skill
  uninstall <id>          Uninstall a skill
  update [id|--all]       Update one or all installed skills
  check                   Check installed skill state`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills",
	Run:   runSkillsList,
}

var skillsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the skills marketplace",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsSearch,
}

var skillsTrendingCmd = &cobra.Command{
	Use:   "trending",
	Short: "Show trending marketplace skills",
	Run:   runSkillsTrending,
}

var skillsInfoCmd = &cobra.Command{
	Use:   "info <id>",
	Short: "Show skill details",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsInfo,
}

var skillsSecurityCmd = &cobra.Command{
	Use:   "security <id>",
	Short: "Show skill security report",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsSecurity,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install <id|github:owner/repo>",
	Short: "Install a skill",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsInstall,
}

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall <id>",
	Short: "Uninstall a skill",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsUninstall,
}

var skillsUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update one skill or all available updates",
	Args:  cobra.MaximumNArgs(1),
	Run:   runSkillsUpdate,
}

var skillsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check installed skills and pending updates",
	Run:   runSkillsCheck,
}

func init() {
	skillsListCmd.Flags().BoolVar(&skillsEligible, "eligible", false, "show only eligible skills")
	skillsListCmd.Flags().BoolVar(&skillsInstalled, "installed", false, "list installed skills from the marketplace")

	skillsSearchCmd.Flags().StringVar(&skillsSearchSort, "sort", "", "sort mode: trending, newest, most_used")
	skillsSearchCmd.Flags().StringVar(&skillsSearchCat, "category", "", "filter by category")
	skillsSearchCmd.Flags().StringVar(&skillsSearchSource, "source", "", "filter by source or source group")
	skillsSearchCmd.Flags().StringVar(&skillsSearchRisk, "risk", "", "filter by security badge: green, yellow, red")
	skillsSearchCmd.Flags().StringVar(&skillsSearchInstallType, "install-type", "", "filter by install type")
	skillsSearchCmd.Flags().StringVar(&skillsSearchArtifact, "artifact", "", "filter by artifact kind")
	skillsSearchCmd.Flags().BoolVar(&skillsSearchSem, "semantic", false, "enable semantic reranking when embeddings are ready")
	skillsSearchCmd.Flags().BoolVar(&skillsSearchSafeOnly, "safe-only", false, "show only green badge skills")
	skillsSearchCmd.Flags().BoolVar(&skillsSearchCurated, "curated", false, "show only curated/featured skills")

	skillsTrendingCmd.Flags().StringVar(&skillsTrendingCat, "category", "", "filter by category")
	skillsTrendingCmd.Flags().StringVar(&skillsTrendingSource, "source", "", "filter by source or source group")
	skillsTrendingCmd.Flags().BoolVar(&skillsTrendingCurated, "curated", false, "show curated/featured skills")
	skillsTrendingCmd.Flags().IntVar(&skillsTrendingLim, "limit", 20, "number of skills to return")

	skillsUpdateCmd.Flags().BoolVar(&skillsUpdateAll, "all", false, "update all pending skills")
	skillsInstallCmd.Flags().BoolVar(&skillsAckRisk, "ack-risk", false, "acknowledge yellow risk warnings and continue installation")
	skillsUpdateCmd.Flags().BoolVar(&skillsAckRisk, "ack-risk", false, "acknowledge yellow risk warnings and continue update")

	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsSearchCmd)
	skillsCmd.AddCommand(skillsTrendingCmd)
	skillsCmd.AddCommand(skillsInfoCmd)
	skillsCmd.AddCommand(skillsSecurityCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsCmd.AddCommand(skillsCheckCmd)

	rootCmd.AddCommand(skillsCmd)
}

type SkillInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Enabled     bool     `json:"enabled"`
	Builtin     bool     `json:"builtin"`
}

type marketplaceSearchResult struct {
	Skill struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Category      string   `json:"category"`
		Tags          []string `json:"tags"`
		LatestVersion string   `json:"latest_version"`
		TrendingScore float64  `json:"trending_score"`
		RiskLevel     string   `json:"risk_level"`
		SecurityBadge string   `json:"security_badge"`
		SourceName    string   `json:"source_name"`
		SourceGroup   string   `json:"source_group"`
		InstallType   string   `json:"install_type"`
		ArtifactKind  string   `json:"artifact_kind"`
		Installable   bool     `json:"installable"`
	} `json:"skill"`
	Score float64 `json:"score"`
}

type marketplaceSearchResponse struct {
	Skills     []marketplaceSearchResult `json:"skills"`
	Total      int                       `json:"total"`
	Page       int                       `json:"page"`
	PageSize   int                       `json:"page_size"`
	TotalPages int                       `json:"total_pages"`
}

type installedSkill struct {
	SkillID           string `json:"skill_id"`
	Name              string `json:"name"`
	InstalledVersion  string `json:"installed_version"`
	Enabled           bool   `json:"enabled"`
	AutoUpdate        bool   `json:"auto_update"`
	LastSecurityScore int    `json:"last_security_score"`
	LatestVersion     string `json:"latest_version"`
	UpdateAvailable   bool   `json:"update_available"`
}

type installedSkillsEnvelope struct {
	Skills []installedSkill `json:"skills"`
	Count  int              `json:"count"`
}

type securityReport struct {
	SkillID             string   `json:"skill_id"`
	Version             string   `json:"version"`
	Score               int      `json:"score"`
	RiskLevel           string   `json:"risk_level"`
	SecurityBadge       string   `json:"security_badge"`
	VulnerabilityStatus string   `json:"vulnerability_status"`
	Permissions         []string `json:"permissions"`
	Secrets             []string `json:"secrets"`
	Vulnerabilities     []string `json:"vulnerabilities"`
	HasPromptInjection  bool     `json:"has_prompt_injection"`
	HasShellInjection   bool     `json:"has_shell_injection"`
	HasDataExfiltration bool     `json:"has_data_exfiltration"`
	InstallSurface      struct {
		InstallType         string   `json:"install_type"`
		ArtifactKind        string   `json:"artifact_kind"`
		Installable         bool     `json:"installable"`
		HasBinary           bool     `json:"has_binary"`
		HasScripts          bool     `json:"has_scripts"`
		DependencyManifests []string `json:"dependency_manifests"`
	} `json:"install_surface"`
	Evidence []struct {
		Type        string `json:"type"`
		Severity    string `json:"severity"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Value       string `json:"value"`
	} `json:"evidence"`
}

type installResult struct {
	SkillID  string         `json:"skill_id"`
	Version  string         `json:"version"`
	Path     string         `json:"path"`
	Warnings []string       `json:"warnings"`
	Security securityReport `json:"security"`
}

type skillDetailResponse struct {
	Skill struct {
		ID                  string   `json:"id"`
		Name                string   `json:"name"`
		Description         string   `json:"description"`
		Author              string   `json:"author"`
		Category            string   `json:"category"`
		Tags                []string `json:"tags"`
		LatestVersion       string   `json:"latest_version"`
		SourceName          string   `json:"source_name"`
		SourceGroup         string   `json:"source_group"`
		Homepage            string   `json:"homepage"`
		DownloadURL         string   `json:"download_url"`
		RiskLevel           string   `json:"risk_level"`
		SecurityBadge       string   `json:"security_badge"`
		InstallType         string   `json:"install_type"`
		ArtifactKind        string   `json:"artifact_kind"`
		Installable         bool     `json:"installable"`
		HasVulnerabilities  bool     `json:"has_vulnerabilities"`
		HasPromptInjection  bool     `json:"has_prompt_injection"`
		HasShellInjection   bool     `json:"has_shell_injection"`
		HasDataExfiltration bool     `json:"has_data_exfiltration"`
	} `json:"skill"`
	Version struct {
		Version string `json:"version"`
	} `json:"version"`
	Security  securityReport `json:"security"`
	Installed bool           `json:"installed"`
	Enabled   bool           `json:"enabled"`
}

type updatesEnvelope struct {
	Updates []struct {
		SkillID        string `json:"skill_id"`
		CurrentVersion string `json:"current_version"`
		LatestVersion  string `json:"latest_version"`
		Action         string `json:"action"`
	} `json:"updates"`
	Count int `json:"count"`
}

func getSkillsBaseURL() string {
	return getServiceAPIBaseURL("/api/v1/skills")
}

func skillsHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func skillsRequestAuthorizationHeader(endpoint string) (string, error) {
	if token := strings.TrimSpace(os.Getenv("BLUE_HARNESS_BEARER_TOKEN")); token != "" {
		return "Bearer " + token, nil
	}
	if !strings.HasPrefix(strings.TrimSpace(endpoint), getSkillsBaseURL()) {
		return "", nil
	}
	return localHarnessAuthorizationHeader()
}

func doSkillsRequest(method, endpoint string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader, err := skillsRequestAuthorizationHeader(endpoint); err == nil && authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return skillsHTTPClient().Do(req)
}

func decodeInto(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var errBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			if msg, ok := errBody["error"].(string); ok {
				return fmt.Errorf("%s", msg)
			}
			if msg, ok := errBody["message"].(string); ok {
				return fmt.Errorf("%s", msg)
			}
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if strings.TrimSpace(string(data)) != "" {
			return fmt.Errorf("%s", strings.TrimSpace(string(data)))
		}
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func runSkillsList(cmd *cobra.Command, args []string) {
	if skillsInstalled {
		resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/installed", nil)
		if err != nil {
			printSkillsError("Failed to fetch installed skills", err)
			return
		}
		var result installedSkillsEnvelope
		if err := decodeInto(resp, &result); err != nil {
			printSkillsError("Failed to parse installed skills", err)
			return
		}
		if jsonOutput {
			printJSON(result)
			return
		}
		if len(result.Skills) == 0 {
			fmt.Println("No installed skills")
			return
		}
		fmt.Println("Installed Skills:")
		for _, skill := range result.Skills {
			updateSuffix := ""
			if skill.UpdateAvailable {
				updateSuffix = " [update available]"
			}
			fmt.Printf("  %s (%s)%s\n", skill.SkillID, skill.InstalledVersion, updateSuffix)
			if skill.Name != "" {
				fmt.Printf("      %s\n", skill.Name)
			}
		}
		fmt.Printf("\nTotal: %d skills\n", result.Count)
		return
	}

	resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL(), nil)
	if err != nil {
		printSkillsError("Failed to fetch skills", err)
		return
	}
	defer resp.Body.Close()

	var envelope struct {
		Skills []SkillInfo `json:"skills"`
	}
	var list []SkillInfo
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		printSkillsError("Failed to read response", err)
		return
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Skills != nil {
		list = envelope.Skills
	} else if err := json.Unmarshal(data, &list); err != nil {
		printSkillsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{"skills": list})
		return
	}
	if len(list) == 0 {
		fmt.Println("No skills available")
		return
	}
	fmt.Println("Available Skills:")
	for _, skill := range list {
		fmt.Printf("  %s", skill.Name)
		if skill.Version != "" {
			fmt.Printf(" (%s)", skill.Version)
		}
		fmt.Println()
		if skill.Description != "" {
			fmt.Printf("      %s\n", skill.Description)
		}
	}
	fmt.Printf("\nTotal: %d skills\n", len(list))
}

func runSkillsSearch(cmd *cobra.Command, args []string) {
	params := url.Values{}
	params.Set("q", args[0])
	if skillsSearchSort != "" {
		params.Set("sort", skillsSearchSort)
	}
	if skillsSearchCat != "" {
		params.Set("category", skillsSearchCat)
	}
	if skillsSearchSource != "" {
		params.Set("sources", skillsSearchSource)
	}
	if skillsSearchRisk != "" {
		params.Set("risk_badges", skillsSearchRisk)
	}
	if skillsSearchInstallType != "" {
		params.Set("install_types", skillsSearchInstallType)
	}
	if skillsSearchArtifact != "" {
		params.Set("artifact_kinds", skillsSearchArtifact)
	}
	if skillsSearchSem {
		params.Set("semantic", "true")
	}
	if skillsSearchSafeOnly {
		params.Set("risk_badges", skillmarket.BadgeGreen)
	}
	if skillsSearchCurated {
		params.Set("curated", "true")
	}
	resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/search?"+params.Encode(), nil)
	if err != nil {
		printSkillsError("Failed to search skills", err)
		return
	}
	var result marketplaceSearchResponse
	if err := decodeInto(resp, &result); err != nil {
		printSkillsError("Failed to parse search results", err)
		return
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	if len(result.Skills) == 0 {
		fmt.Println("No matching skills found")
		return
	}
	fmt.Println("Search Results:")
	for _, item := range result.Skills {
		fmt.Printf("  %s (%s)\n", item.Skill.ID, item.Skill.LatestVersion)
		fmt.Printf("      %s\n", item.Skill.Name)
		if item.Skill.Description != "" {
			fmt.Printf("      %s\n", item.Skill.Description)
		}
		fmt.Printf("      score=%.2f badge=%s risk=%s category=%s source=%s\n", item.Score, item.Skill.SecurityBadge, item.Skill.RiskLevel, displaySkillCategory(item.Skill.Category), firstNonEmpty(item.Skill.SourceName, item.Skill.SourceGroup))
		fmt.Printf("      install=%t type=%s artifact=%s\n", item.Skill.Installable, item.Skill.InstallType, item.Skill.ArtifactKind)
	}
}

func runSkillsTrending(cmd *cobra.Command, args []string) {
	if skillsTrendingSource != "" || skillsTrendingCurated {
		params := url.Values{}
		params.Set("page", "1")
		if skillsTrendingLim > 0 {
			params.Set("page_size", fmt.Sprintf("%d", skillsTrendingLim))
		}
		if skillsTrendingCat != "" {
			params.Set("category", skillsTrendingCat)
		}
		if skillsTrendingSource != "" {
			params.Set("sources", skillsTrendingSource)
		}
		if skillsTrendingCurated {
			params.Set("curated", "true")
			params.Set("sort", "featured")
		}
		resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/search?"+params.Encode(), nil)
		if err != nil {
			printSkillsError("Failed to fetch trending skills", err)
			return
		}
		var result marketplaceSearchResponse
		if err := decodeInto(resp, &result); err != nil {
			printSkillsError("Failed to parse trending response", err)
			return
		}
		if jsonOutput {
			printJSON(result)
			return
		}
		fmt.Println("Trending Skills:")
		for _, item := range result.Skills {
			fmt.Printf("  %s\n", item.Skill.ID)
			fmt.Printf("      %s\n", item.Skill.Name)
			fmt.Printf("      badge=%s source=%s score=%.2f\n", item.Skill.SecurityBadge, firstNonEmpty(item.Skill.SourceName, item.Skill.SourceGroup), item.Score)
		}
		return
	}
	params := url.Values{}
	if skillsTrendingCat != "" {
		params.Set("category", skillsTrendingCat)
	}
	if skillsTrendingLim > 0 {
		params.Set("limit", fmt.Sprintf("%d", skillsTrendingLim))
	}
	resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/trending?"+params.Encode(), nil)
	if err != nil {
		printSkillsError("Failed to fetch trending skills", err)
		return
	}
	var result struct {
		Skills []map[string]interface{} `json:"skills"`
		Count  int                      `json:"count"`
	}
	if err := decodeInto(resp, &result); err != nil {
		printSkillsError("Failed to parse trending response", err)
		return
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	if len(result.Skills) == 0 {
		fmt.Println("No trending skills")
		return
	}
	fmt.Println("Trending Skills:")
	for _, item := range result.Skills {
		fmt.Printf("  %v\n", item["id"])
		if name, ok := item["name"]; ok {
			fmt.Printf("      %v\n", name)
		}
		if score, ok := item["trending_score"]; ok {
			fmt.Printf("      trending=%v\n", score)
		}
	}
}

func runSkillsInfo(cmd *cobra.Command, args []string) {
	skillID := args[0]
	resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/"+skillID, nil)
	if err != nil {
		printSkillsError("Failed to fetch skill", err)
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		if jsonOutput {
			printJSON(map[string]interface{}{"error": "Skill not found", "id": skillID})
		} else {
			fmt.Printf("Skill not found: %s\n", skillID)
		}
		os.Exit(1)
	}
	var skill skillDetailResponse
	if err := decodeInto(resp, &skill); err != nil {
		printSkillsError("Failed to parse response", err)
		return
	}
	if jsonOutput {
		printJSON(skill)
		return
	}
	fmt.Printf("Skill: %s\n", skill.Skill.Name)
	fmt.Printf("ID: %s\n", skill.Skill.ID)
	version := firstNonEmpty(skill.Version.Version, skill.Skill.LatestVersion)
	if version != "" {
		fmt.Printf("Version: %s\n", version)
	}
	if skill.Skill.Category != "" {
		fmt.Printf("Category: %s\n", displaySkillCategory(skill.Skill.Category))
	}
	if skill.Skill.Description != "" {
		fmt.Printf("Description: %s\n", skill.Skill.Description)
	}
	fmt.Printf("Installed: %v\n", skill.Installed)
	fmt.Printf("Enabled: %v\n", skill.Enabled)
	fmt.Printf("Source: %s\n", firstNonEmpty(skill.Skill.SourceName, skill.Skill.SourceGroup))
	fmt.Printf("Security: %s (%s)\n", skill.Skill.SecurityBadge, skill.Skill.RiskLevel)
	fmt.Printf("Install: %t (%s / %s)\n", skill.Skill.Installable, skill.Skill.InstallType, skill.Skill.ArtifactKind)
	if len(skill.Skill.Tags) > 0 {
		fmt.Printf("Tags: %v\n", skill.Skill.Tags)
	}
	if skill.Skill.Homepage != "" {
		fmt.Printf("Homepage: %s\n", skill.Skill.Homepage)
	}
}

func runSkillsSecurity(cmd *cobra.Command, args []string) {
	resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/security/"+args[0], nil)
	if err != nil {
		printSkillsError("Failed to fetch security report", err)
		return
	}
	var report securityReport
	if err := decodeInto(resp, &report); err != nil {
		printSkillsError("Failed to parse security report", err)
		return
	}
	if jsonOutput {
		printJSON(report)
		return
	}
	fmt.Printf("Skill: %s\n", report.SkillID)
	fmt.Printf("Version: %s\n", report.Version)
	fmt.Printf("Score: %d\n", report.Score)
	fmt.Printf("Risk: %s (%s)\n", report.RiskLevel, report.SecurityBadge)
	fmt.Printf("Install: %t (%s / %s)\n", report.InstallSurface.Installable, report.InstallSurface.InstallType, report.InstallSurface.ArtifactKind)
	fmt.Printf("Vulnerabilities: %s\n", report.VulnerabilityStatus)
	fmt.Printf("Prompt injection: %v\n", report.HasPromptInjection)
	fmt.Printf("Shell injection: %v\n", report.HasShellInjection)
	fmt.Printf("Data exfiltration: %v\n", report.HasDataExfiltration)
	if len(report.Permissions) > 0 {
		fmt.Printf("Permissions: %s\n", strings.Join(report.Permissions, ", "))
	}
	if len(report.Secrets) > 0 {
		fmt.Printf("Secrets: %s\n", strings.Join(report.Secrets, ", "))
	}
	if len(report.Vulnerabilities) > 0 {
		fmt.Printf("Findings: %s\n", strings.Join(report.Vulnerabilities, ", "))
	}
	if len(report.Evidence) > 0 {
		fmt.Println("Evidence:")
		for _, item := range report.Evidence {
			line := fmt.Sprintf("  - [%s] %s", item.Severity, item.Title)
			if item.Description != "" {
				line += ": " + item.Description
			}
			if item.Value != "" {
				line += " (" + item.Value + ")"
			}
			fmt.Println(line)
		}
	}
}

func displaySkillCategory(value string) string {
	switch strings.TrimSpace(value) {
	case "ai_intelligence":
		return "AI Intelligence"
	case "development_tools", "development", "integration", "extension":
		return "Development Tools"
	case "productivity", "utility":
		return "Productivity"
	case "data_analysis", "analytics", "information":
		return "Data Analysis"
	case "content_creation":
		return "Content Creation"
	case "security_compliance", "system":
		return "Security & Compliance"
	case "communication_collaboration", "communication":
		return "Communication & Collaboration"
	default:
		return value
	}
}

func runSkillsInstall(cmd *cobra.Command, args []string) {
	target := args[0]
	payload := map[string]interface{}{}
	if strings.HasPrefix(target, "github:") {
		payload["github"] = strings.TrimPrefix(target, "github:")
	} else {
		payload["id"] = target
	}
	if skillsAckRisk {
		payload["ack_risk"] = true
	}
	resp, err := doSkillsRequest(http.MethodPost, getSkillsBaseURL()+"/install", payload)
	if err != nil {
		printSkillsError("Failed to install skill", err)
		return
	}
	var result installResult
	if err := decodeInto(resp, &result); err != nil {
		if strings.Contains(err.Error(), "risk acknowledgement") {
			if skillsAckRisk {
				printSkillsError("Failed to install skill", err)
				return
			}
			if promptRiskAcknowledgement(target) {
				payload["ack_risk"] = true
				resp, err = doSkillsRequest(http.MethodPost, getSkillsBaseURL()+"/install", payload)
				if err != nil {
					printSkillsError("Failed to install skill", err)
					return
				}
				if err := decodeInto(resp, &result); err != nil {
					printSkillsError("Failed to parse install response", err)
					return
				}
			} else {
				printSkillsError("Installation cancelled", err)
				return
			}
		} else {
			printSkillsError("Failed to parse install response", err)
			return
		}
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	fmt.Printf("Installed %s (%s)\n", result.SkillID, result.Version)
	fmt.Printf("Path: %s\n", result.Path)
	if len(result.Warnings) > 0 {
		fmt.Println("Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
}

func runSkillsUninstall(cmd *cobra.Command, args []string) {
	resp, err := doSkillsRequest(http.MethodPost, getSkillsBaseURL()+"/"+args[0]+"/uninstall", nil)
	if err != nil {
		printSkillsError("Failed to uninstall skill", err)
		return
	}
	var result map[string]interface{}
	if err := decodeInto(resp, &result); err != nil {
		printSkillsError("Failed to parse uninstall response", err)
		return
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	fmt.Printf("Uninstalled %s\n", args[0])
}

func runSkillsUpdate(cmd *cobra.Command, args []string) {
	if skillsUpdateAll {
		resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/updates", nil)
		if err != nil {
			printSkillsError("Failed to fetch pending updates", err)
			return
		}
		var updates updatesEnvelope
		if err := decodeInto(resp, &updates); err != nil {
			printSkillsError("Failed to parse pending updates", err)
			return
		}
		if len(updates.Updates) == 0 {
			fmt.Println("No pending updates")
			return
		}
		for _, item := range updates.Updates {
			resp, err := doSkillsRequest(http.MethodPost, getSkillsBaseURL()+"/"+item.SkillID+"/update", nil)
			if err != nil {
				printSkillsError("Failed to update skill", err)
				return
			}
			var result installResult
			if err := decodeInto(resp, &result); err != nil {
				printSkillsError("Failed to parse update response", err)
				return
			}
			fmt.Printf("Updated %s -> %s\n", result.SkillID, result.Version)
		}
		return
	}
	if len(args) == 0 {
		resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/updates", nil)
		if err != nil {
			printSkillsError("Failed to fetch pending updates", err)
			return
		}
		var updates updatesEnvelope
		if err := decodeInto(resp, &updates); err != nil {
			printSkillsError("Failed to parse pending updates", err)
			return
		}
		if jsonOutput {
			printJSON(updates)
			return
		}
		if len(updates.Updates) == 0 {
			fmt.Println("No pending updates")
			return
		}
		fmt.Println("Pending Updates:")
		for _, item := range updates.Updates {
			fmt.Printf("  %s: %s -> %s (%s)\n", item.SkillID, item.CurrentVersion, item.LatestVersion, item.Action)
		}
		return
	}
	endpoint := getSkillsBaseURL() + "/" + args[0] + "/update"
	if skillsAckRisk {
		endpoint += "?ack_risk=true"
	}
	resp, err := doSkillsRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		printSkillsError("Failed to update skill", err)
		return
	}
	var result installResult
	if err := decodeInto(resp, &result); err != nil {
		if strings.Contains(err.Error(), "risk acknowledgement") && !skillsAckRisk && promptRiskAcknowledgement(args[0]) {
			resp, err = doSkillsRequest(http.MethodPost, getSkillsBaseURL()+"/"+args[0]+"/update?ack_risk=true", nil)
			if err != nil {
				printSkillsError("Failed to update skill", err)
				return
			}
			if err := decodeInto(resp, &result); err != nil {
				printSkillsError("Failed to parse update response", err)
				return
			}
		} else {
			printSkillsError("Failed to parse update response", err)
			return
		}
	}
	if jsonOutput {
		printJSON(result)
		return
	}
	fmt.Printf("Updated %s -> %s\n", result.SkillID, result.Version)
}

func runSkillsCheck(cmd *cobra.Command, args []string) {
	resp, err := doSkillsRequest(http.MethodGet, getSkillsBaseURL()+"/installed", nil)
	if err != nil {
		printSkillsError("Failed to check skills", err)
		return
	}
	var installed installedSkillsEnvelope
	if err := decodeInto(resp, &installed); err != nil {
		printSkillsError("Failed to parse installed skills", err)
		return
	}
	if jsonOutput {
		printJSON(installed)
		return
	}
	fmt.Println("Installed Skill Status:")
	fmt.Printf("  Total: %d\n", installed.Count)
	pending := 0
	for _, skill := range installed.Skills {
		if skill.UpdateAvailable {
			pending++
		}
	}
	fmt.Printf("  Updates available: %d\n", pending)
	if installed.Count == 0 {
		fmt.Println("\nNo installed skills")
	}
}

func printSkillsError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   msg,
			"details": err.Error(),
		})
	} else {
		if noColor {
			fmt.Printf("Error: %s\n", msg)
		} else {
			fmt.Printf("\033[31mError:\033[0m %s\n", msg)
		}
		if err != nil && (verbose || !term.IsTerminal(int(os.Stdout.Fd()))) {
			fmt.Printf("Details: %v\n", err)
		}
	}
	skillsExit(1)
}

func promptRiskAcknowledgement(target string) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}
	fmt.Printf("Skill %s is marked yellow risk. Continue installation? [y/N]: ", target)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
