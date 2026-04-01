package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
)

func renderSkillMarketError(c echo.Context, err error, fallbackCode int) error {
	if err == nil {
		return nil
	}
	if httpErr, ok := err.(*echo.HTTPError); ok {
		message := strings.TrimSpace(fmt.Sprint(httpErr.Message))
		if message == "" {
			message = strings.TrimSpace(err.Error())
		}
		return c.JSON(httpErr.Code, map[string]string{"error": message})
	}
	return c.JSON(fallbackCode, map[string]string{"error": err.Error()})
}

func (h *SkillHandler) marketUnavailable(c echo.Context) error {
	return c.JSON(http.StatusServiceUnavailable, map[string]string{
		"error": "skill marketplace not initialized",
	})
}

type marketAdviceRequest struct {
	Query             string              `json:"query"`
	InstalledDecision *agentcore.Decision `json:"installed_decision,omitempty"`
}

type marketAdviceResponse struct {
	*skilladvisor.Advice
	SkillSelectorError string `json:"skill_selector_error,omitempty"`
	SearchError        string `json:"search_error,omitempty"`
}

func marketAdviceErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (h *SkillHandler) MarketAdviseSkills(c echo.Context) error {
	var req marketAdviceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	h.mu.RLock()
	advisor := h.skillAdvisor
	selector := h.skillSelector
	selectOptions := h.selectOptions
	h.mu.RUnlock()
	if advisor == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skill advisor not initialized"})
	}

	var selectorErr string
	selectedDecision := req.InstalledDecision
	if selectedDecision == nil && selector != nil {
		opts := agentcore.SelectOptions{}
		if selectOptions != nil {
			opts = selectOptions()
		}
		decision, err := selector.Select(c.Request().Context(), req.Query, opts)
		if err != nil {
			selectorErr = err.Error()
		} else {
			copied := decision
			selectedDecision = &copied
		}
	}

	advice, err := advisor.Advise(c.Request().Context(), req.Query, selectedDecision)
	if advice == nil {
		advice = &skilladvisor.Advice{Query: req.Query, InstalledDecision: selectedDecision}
	}

	return c.JSON(http.StatusOK, marketAdviceResponse{
		Advice:             advice,
		SkillSelectorError: selectorErr,
		SearchError:        marketAdviceErrorString(err),
	})
}

func (h *SkillHandler) MarketSearchSkills(c echo.Context) error {
	// Rate limiting: 5 requests per second per IP
	ip := c.RealIP()
	now := time.Now()

	h.rateLimitMu.Lock()
	entry, exists := h.rateLimits[ip]
	if !exists || now.After(entry.reset) {
		// First request or window expired, create new entry
		h.rateLimits[ip] = &rateLimitEntry{
			count: 1,
			reset: now.Add(time.Second),
		}
	} else {
		entry.count++
		if entry.count > 5 {
			h.rateLimitMu.Unlock()
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded: 5 requests per second",
			})
		}
	}
	h.rateLimitMu.Unlock()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	semantic := c.QueryParam("semantic") == "true" || c.QueryParam("semantic") == "1"
	query := skillmarket.SearchQuery{
		Query:               c.QueryParam("q"),
		Category:            c.QueryParam("category"),
		Categories:          parseCSV(c.QueryParam("categories")),
		Sources:             parseCSV(c.QueryParam("sources")),
		RiskBadges:          parseCSV(c.QueryParam("risk_badges")),
		InstallTypes:        parseCSV(c.QueryParam("install_types")),
		ArtifactKinds:       parseCSV(c.QueryParam("artifact_kinds")),
		Sort:                firstNonEmpty(c.QueryParam("sort"), c.QueryParam("sort_by")),
		Page:                page,
		PageSize:            pageSize,
		Semantic:            semantic,
		Installable:         parseBoolPtr(c.QueryParam("installable")),
		Curated:             parseBoolPtr(c.QueryParam("curated")),
		OpenSourceOnly:      parseTruthy(c.QueryParam("open_source_only")),
		HasVulnerabilities:  parseBoolPtr(c.QueryParam("has_vulnerabilities")),
		HasPromptInjection:  parseBoolPtr(c.QueryParam("has_prompt_injection")),
		HasShellInjection:   parseBoolPtr(c.QueryParam("has_shell_injection")),
		HasDataExfiltration: parseBoolPtr(c.QueryParam("has_data_exfiltration")),
	}
	market, err := h.ensureMarketplace()
	if err != nil || market == nil {
		c.Response().Header().Set("Cache-Control", "max-age=5, public")
		return c.JSON(http.StatusOK, h.fallbackMarketSearch(c.Request().Context(), query))
	}
	result, err := market.Search(c.Request().Context(), query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	c.Response().Header().Set("Cache-Control", "max-age=5, public")
	return c.JSON(http.StatusOK, result)
}

func (h *SkillHandler) MarketTrendingSkills(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	market, err := h.ensureMarketplace()
	if err != nil || market == nil {
		resp := h.fallbackMarketSearch(c.Request().Context(), skillmarket.SearchQuery{
			Category: c.QueryParam("category"),
			Sort:     "trending",
			Page:     1,
			PageSize: limit,
		})
		skills := make([]*RemoteSkill, 0, len(resp.Skills))
		for _, item := range resp.Skills {
			skills = append(skills, item.Skill)
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills": skills,
			"count":  len(skills),
		})
	}
	skills, err := market.Trending(c.Request().Context(), c.QueryParam("category"), limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"skills": skills,
		"count":  len(skills),
	})
}

func (h *SkillHandler) MarketFeaturedSkills(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	market, err := h.ensureMarketplace()
	if err != nil || market == nil {
		resp := h.fallbackMarketSearch(c.Request().Context(), skillmarket.SearchQuery{
			Category: c.QueryParam("category"),
			Sources:  []string{c.QueryParam("source")},
			Sort:     "featured",
			Page:     1,
			PageSize: limit,
		})
		skills := make([]*RemoteSkill, 0, len(resp.Skills))
		for _, item := range resp.Skills {
			skills = append(skills, item.Skill)
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills": skills,
			"count":  len(skills),
		})
	}
	skills, err := market.Featured(c.Request().Context(), c.QueryParam("category"), c.QueryParam("source"), limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"skills": skills,
		"count":  len(skills),
	})
}

func (h *SkillHandler) MarketFilters(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil || market == nil {
		return c.JSON(http.StatusOK, h.fallbackMarketFilters(c.Request().Context()))
	}
	filters, err := market.Filters(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, filters)
}

func (h *SkillHandler) MarketSecurityReport(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if market == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "security report not found"})
	}
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	report, err := market.GetSecurity(c.Request().Context(), id, c.QueryParam("version"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if report == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "security report not found"})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *SkillHandler) MarketInstallSkill(c echo.Context) error {
	var req skillmarket.InstallRequest
	if c.Request().ContentLength > 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
	}
	if !req.AckRisk {
		req.AckRisk = parseTruthy(c.QueryParam("ack_risk"))
	}
	market, err := h.ensureMarketplace()
	if market == nil {
		if strings.TrimSpace(req.GitHub) != "" {
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "direct github install requires the skill marketplace service"})
		}
		result, err := h.legacyMarketInstall(c.Request().Context(), req.ID, getUserIDFromContext(c))
		if err != nil {
			return renderSkillMarketError(c, err, http.StatusBadRequest)
		}
		return c.JSON(http.StatusOK, result)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	result, err := market.Install(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *SkillHandler) MarketInstalledSkills(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil || market == nil {
		skills := h.fallbackInstalledSkills()
		return c.JSON(http.StatusOK, map[string]interface{}{
			"skills": skills,
			"count":  len(skills),
		})
	}
	skills, err := market.ListInstalled(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"skills": skills,
		"count":  len(skills),
	})
}

func (h *SkillHandler) MarketUninstallSkill(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if resolvedID, ok := h.resolveInstalledSkillID(id); ok {
		id = resolvedID
	}
	if info := h.registry.GetInfo(id); info != nil && info.Builtin {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "cannot uninstall builtin skill"})
	}
	market, marketErr := h.ensureMarketplace()
	if market == nil {
		if err := h.legacyMarketUninstall(id); err != nil {
			return renderSkillMarketError(c, err, http.StatusBadRequest)
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":  true,
			"skill_id": id,
		})
	}
	if marketErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": marketErr.Error()})
	}
	if err := market.Uninstall(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"skill_id": id,
	})
}

func (h *SkillHandler) MarketUpdateSkill(c echo.Context) error {
	id, err := validatedSkillID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	market, err := h.ensureMarketplace()
	if market == nil {
		result, err := h.legacyMarketUpdate(c.Request().Context(), id, getUserIDFromContext(c))
		if err != nil {
			return renderSkillMarketError(c, err, http.StatusBadRequest)
		}
		return c.JSON(http.StatusOK, result)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	result, err := market.Install(c.Request().Context(), skillmarket.InstallRequest{
		ID:      id,
		AckRisk: parseTruthy(c.QueryParam("ack_risk")),
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *SkillHandler) MarketDiscoverSkills(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if market == nil {
		if h.syncService != nil {
			h.syncService.ForceSyncAll(context.Background())
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"sources_processed": len(h.sources),
			"discovered":        0,
			"updated":           0,
			"failed":            0,
		})
	}
	status, started := market.StartDiscoverAsync()
	code := http.StatusAccepted
	message := "discover started"
	if !started {
		message = "discover already running"
	}
	return c.JSON(code, map[string]interface{}{
		"accepted":            started,
		"running":             status.Running,
		"started_at":          status.StartedAt,
		"finished_at":         status.FinishedAt,
		"last_error":          status.LastError,
		"total_sources":       status.TotalSources,
		"processed_sources":   status.ProcessedSources,
		"current_source_id":   status.CurrentSourceID,
		"current_source_name": status.CurrentSourceName,
		"source_results":      status.SourceResults,
		"result":              status.Result,
		"message":             message,
	})
}

func (h *SkillHandler) MarketDiscoverStatus(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if market == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"running": false,
		})
	}
	status := market.GetDiscoverStatus()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"running":             status.Running,
		"started_at":          status.StartedAt,
		"finished_at":         status.FinishedAt,
		"last_error":          status.LastError,
		"total_sources":       status.TotalSources,
		"processed_sources":   status.ProcessedSources,
		"current_source_id":   status.CurrentSourceID,
		"current_source_name": status.CurrentSourceName,
		"source_results":      status.SourceResults,
		"result":              status.Result,
	})
}

func (h *SkillHandler) MarketEmbeddingStatus(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if market == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"running": false,
		})
	}
	status := market.GetEmbeddingStatus()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"running":            status.Running,
		"started_at":         status.StartedAt,
		"finished_at":        status.FinishedAt,
		"last_error":         status.LastError,
		"total_skills":       status.TotalSkills,
		"processed_skills":   status.ProcessedSkills,
		"embedded_skills":    status.EmbeddedSkills,
		"failed_skills":      status.FailedSkills,
		"current_skill_id":   status.CurrentSkillID,
		"current_skill_name": status.CurrentSkillName,
		"phase":              status.Phase,
	})
}

func (h *SkillHandler) MarketListUpdates(c echo.Context) error {
	market, err := h.ensureMarketplace()
	if err != nil || market == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"updates": []interface{}{},
			"count":   0,
		})
	}
	updates, err := market.ListUpdates(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"updates": updates,
		"count":   len(updates),
	})
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseTruthy(raw string) bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseBoolPtr(raw string) *bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "":
		return nil
	case "1", "true", "yes", "y", "on":
		value := true
		return &value
	case "0", "false", "no", "n", "off":
		value := false
		return &value
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type fallbackMarketSearchResult struct {
	Skill         *RemoteSkill `json:"skill"`
	Score         float64      `json:"score"`
	KeywordScore  float64      `json:"keyword_score,omitempty"`
	SemanticScore float64      `json:"semantic_score,omitempty"`
	MatchSource   string       `json:"match_source,omitempty"`
}

type fallbackMarketSearchResponse struct {
	Skills     []fallbackMarketSearchResult `json:"skills"`
	Total      int                          `json:"total"`
	Page       int                          `json:"page"`
	PageSize   int                          `json:"page_size"`
	TotalPages int                          `json:"total_pages"`
}

func (h *SkillHandler) fallbackMarketSearch(ctx context.Context, query skillmarket.SearchQuery) fallbackMarketSearchResponse {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 24
	}

	results, err := h.collectFallbackMarketResults(ctx, query)
	if err != nil {
		return fallbackMarketSearchResponse{Skills: []fallbackMarketSearchResult{}, Total: 0, Page: page, PageSize: pageSize, TotalPages: 0}
	}
	total := len(results)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return fallbackMarketSearchResponse{
		Skills:     results[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

func (h *SkillHandler) collectFallbackMarketResults(ctx context.Context, query skillmarket.SearchQuery) ([]fallbackMarketSearchResult, error) {
	if h.store != nil {
		categories := append([]string{}, query.Categories...)
		if strings.TrimSpace(query.Category) != "" {
			categories = append(categories, query.Category)
		}
		opts := skillstore.SearchOptions{
			Query:      query.Query,
			Categories: categories,
			Sources:    compactNonEmpty(query.Sources),
			Page:       1,
			PageSize:   100,
		}
		switch query.Sort {
		case "newest":
			opts.SortBy = "updated"
		case "most_used", "trending", "featured":
			opts.SortBy = "downloads"
		case "name":
			opts.SortBy = "name"
			opts.SortOrder = "asc"
		default:
			opts.SortBy = "relevance"
		}
		results := make([]fallbackMarketSearchResult, 0, opts.PageSize)
		installedIDs := h.getInstalledSkillIDs()
		for {
			search, err := h.store.Search(ctx, opts)
			if err != nil {
				return nil, err
			}
			for _, item := range search.Skills {
				skill := h.skillToRemoteSkill(&item.Skill, installedIDs)
				if !matchesFallbackMarketFilters(skill, query) {
					continue
				}
				results = append(results, fallbackMarketSearchResult{
					Skill:        skill,
					Score:        item.Score,
					KeywordScore: item.Score,
					MatchSource:  "keyword",
				})
			}
			if search.TotalPages <= 0 || opts.Page >= search.TotalPages || len(search.Skills) == 0 {
				break
			}
			opts.Page++
		}
		sortFallbackMarketResults(results, query.Sort)
		return results, nil
	}

	items := h.getFeaturedSkillsFallback()
	if len(items) == 0 {
		h.mu.RLock()
		items = make([]*RemoteSkill, 0, len(h.remoteSkills))
		for _, skill := range h.remoteSkills {
			items = append(items, skill)
		}
		h.mu.RUnlock()
	}
	results := make([]fallbackMarketSearchResult, 0, len(items))
	for _, skill := range items {
		if !matchesFallbackMarketFilters(skill, query) {
			continue
		}
		score := float64(skill.Stars) + (float64(skill.Downloads) * 0.01)
		results = append(results, fallbackMarketSearchResult{
			Skill:        skill,
			Score:        score,
			KeywordScore: score,
			MatchSource:  "fallback",
		})
	}
	sortFallbackMarketResults(results, query.Sort)
	return results, nil
}

func (h *SkillHandler) fallbackMarketFilters(ctx context.Context) *skillmarket.SkillFilters {
	filters := &skillmarket.SkillFilters{
		Installable:     map[string]int{"true": 0, "false": 0},
		SecuritySignals: map[string]int{"vulnerabilities": 0, "prompt_injection": 0, "shell_injection": 0, "data_exfiltration": 0},
	}
	results, _ := h.collectFallbackMarketResults(ctx, skillmarket.SearchQuery{Page: 1, PageSize: 100})

	categoryCounts := make(map[string]int)
	sourceCounts := make(map[string]int)
	riskCounts := make(map[string]int)
	installTypeCounts := make(map[string]int)
	artifactCounts := make(map[string]int)

	if h.store != nil {
		if categories, err := h.store.GetCategories(ctx); err == nil {
			for _, category := range categories {
				categoryCounts[skillmarket.NormalizeMarketplaceCategory(category, "", nil)]++
			}
		}
	}
	for _, item := range results {
		skill := item.Skill
		if skill == nil {
			continue
		}
		if category := strings.TrimSpace(skill.Category); category != "" {
			categoryCounts[skillmarket.NormalizeMarketplaceCategory(category, skill.Description, skill.Tags)]++
		}
		if source := strings.TrimSpace(firstString(skill.SourceName, skill.SourceGroup, skill.SourceID)); source != "" {
			sourceCounts[source]++
		}
		if badge := strings.TrimSpace(skill.SecurityBadge); badge != "" {
			riskCounts[badge]++
		}
		if installType := strings.TrimSpace(skill.InstallType); installType != "" {
			installTypeCounts[installType]++
		}
		if artifact := strings.TrimSpace(skill.ArtifactKind); artifact != "" {
			artifactCounts[artifact]++
		}
		if skill.Installable {
			filters.Installable["true"]++
		} else {
			filters.Installable["false"]++
		}
	}

	filters.Categories = filterOptionsFromCounts(categoryCounts)
	sort.Slice(filters.Categories, func(i, j int) bool {
		left := skillmarket.MarketplaceCategoryOrder(filters.Categories[i].Value)
		right := skillmarket.MarketplaceCategoryOrder(filters.Categories[j].Value)
		if left != right {
			return left < right
		}
		return filters.Categories[i].Value < filters.Categories[j].Value
	})
	filters.Sources = filterOptionsFromCounts(sourceCounts)
	filters.RiskBadges = filterOptionsFromCounts(riskCounts)
	filters.InstallTypes = filterOptionsFromCounts(installTypeCounts)
	filters.ArtifactKinds = filterOptionsFromCounts(artifactCounts)
	return filters
}

func (h *SkillHandler) fallbackInstalledSkills() []map[string]interface{} {
	seen := make(map[string]struct{})
	items := make([]map[string]interface{}, 0)
	if h.localScanner != nil {
		for _, skill := range h.localScanner.GetAll() {
			id := strings.TrimSpace(skill.ID)
			if info := h.registry.GetInfo(id); info != nil && info.Manifest != nil {
				id = strings.TrimSpace(info.Manifest.ID)
			}
			if id == "" {
				id = strings.TrimSpace(skill.ID)
			}
			seen[id] = struct{}{}
			enabled := true
			if info := h.registry.GetInfo(id); info != nil {
				enabled = info.Enabled
			}
			items = append(items, map[string]interface{}{
				"skill_id":            id,
				"name":                firstNonEmpty(skill.Name, id),
				"installed_version":   firstNonEmpty(skill.Version, "unknown"),
				"checksum":            "",
				"source_url":          "",
				"enabled":             enabled,
				"auto_update":         false,
				"installed_at":        skill.DiscoveredAt,
				"updated_at":          skill.LastModified,
				"last_security_score": 0,
				"latest_version":      skill.Version,
				"update_available":    false,
			})
		}
	}
	for _, info := range h.registry.List() {
		if info == nil || info.Manifest == nil {
			continue
		}
		if _, ok := seen[info.Manifest.ID]; ok {
			continue
		}
		items = append(items, map[string]interface{}{
			"skill_id":            info.Manifest.ID,
			"name":                info.Manifest.Name,
			"installed_version":   firstNonEmpty(info.Manifest.Version, "unknown"),
			"checksum":            "",
			"source_url":          "",
			"enabled":             info.Enabled,
			"auto_update":         false,
			"last_security_score": 0,
			"latest_version":      info.Manifest.Version,
			"update_available":    false,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return strings.TrimSpace(items[i]["skill_id"].(string)) < strings.TrimSpace(items[j]["skill_id"].(string))
	})
	return items
}

func legacyInstallResult(id, path string, rs *RemoteSkill) *skillmarket.InstallResult {
	return &skillmarket.InstallResult{
		SkillID:   id,
		Version:   firstNonEmpty(rs.Version, "legacy"),
		Path:      path,
		CachePath: "",
		Security: &skillmarket.SecurityReport{
			SkillID:             id,
			Version:             firstNonEmpty(rs.Version, "legacy"),
			Score:               70,
			RiskLevel:           firstNonEmpty(rs.RiskLevel, "unknown"),
			SecurityBadge:       firstNonEmpty(rs.SecurityBadge, skillmarket.BadgeYellow),
			VulnerabilityStatus: skillmarket.VulnerabilityStatusUnknown,
			InstallSurface: skillmarket.InstallSurface{
				InstallType:  legacyInstallTypeFromURLs(rs.Homepage, rs.DownloadURL),
				ArtifactKind: skillmarket.ArtifactKindUnknown,
				Installable:  true,
			},
			ScannerVersion: skillmarket.ScannerVersion,
			LLMStatus:      "skipped",
		},
		InstalledAt: time.Now(),
	}
}

func (h *SkillHandler) materializeLegacyRemoteSkill(ctx context.Context, id string, rs *RemoteSkill, destDir string, progressFn func(downloaded, total int)) (*skill.Manifest, error) {
	ghURL := firstString(rs.Homepage, rs.DownloadURL)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	if isGitHubDirURL(ghURL) {
		if err := h.downloadGitHubDirectory(ctx, ghURL, destDir, progressFn); err != nil {
			return nil, err
		}
		entryDoc, skillContent, err := readInstalledSkillEntry(destDir)
		if err != nil {
			return nil, err
		}
		if _, err := skillbundle.EnsureCompatibilitySkillDoc(destDir, entryDoc.Path); err != nil {
			return nil, err
		}
		if _, _, err := h.parseSkillContent(string(skillContent), ghURL, rs.Name, rs.Description); err != nil {
			return nil, err
		}
	} else {
		data, err := h.downloadSkillMD(ctx, id, rs)
		if err != nil {
			return nil, err
		}
		entryName := entryDocumentNameFromURL(firstString(rs.DownloadURL, rs.Homepage))
		if _, err := writeInstalledSkillDocument(destDir, entryName, data); err != nil {
			return nil, err
		}
	}

	skillContent, err := readInstalledSkillMarkdown(destDir)
	if err != nil {
		return nil, err
	}
	_, manifest, err := h.parseSkillContent(string(skillContent), firstString(ghURL, rs.DownloadURL), rs.Name, rs.Description)
	if err != nil {
		return nil, err
	}
	manifest.ID = id
	return manifest, nil
}

func (h *SkillHandler) legacyMarketInstall(ctx context.Context, id, userID string) (*skillmarket.InstallResult, error) {
	var err error
	id, err = validatedSkillID(id)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if h.skillsDir == "" {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "skills directory not configured")
	}

	if err := h.ensureSkillInstallTargetAvailable(id); err != nil {
		if httpErr, ok := err.(*echo.HTTPError); ok {
			return nil, httpErr
		}
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	skillDir := filepath.Join(h.skillsDir, id)

	rs := h.findRemoteSkill(ctx, id)
	if rs == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "skill not found in store")
	}

	h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
		"id": id, "percent": 0, "message": "Starting install...",
	})

	tempDir, err := os.MkdirTemp(h.skillsDir, ".skill-install-*")
	if err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	cleanupDir := tempDir
	defer func() {
		if cleanupDir != "" {
			_ = os.RemoveAll(cleanupDir)
		}
	}()

	manifest, err := h.materializeLegacyRemoteSkill(ctx, id, rs, tempDir, func(downloaded, total int) {
		if total <= 0 {
			return
		}
		pct := int(float64(downloaded) / float64(maxInt(total, 1)) * 100)
		h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
			"id": id, "percent": pct, "message": "Downloading files...",
		})
	})
	if err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}

	if err := os.Rename(tempDir, skillDir); err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	cleanupDir = ""
	if err := h.registerInstalledSkill(manifest); err != nil {
		_ = os.RemoveAll(skillDir)
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	h.publishEvent(userID, "skill.install.complete", map[string]interface{}{"id": id})

	result := legacyInstallResult(id, skillDir, rs)
	result.Warnings = append(result.Warnings, manifestValidationWarnings(manifest)...)
	return result, nil
}

func (h *SkillHandler) legacyMarketUpdate(ctx context.Context, id, userID string) (*skillmarket.InstallResult, error) {
	validatedID, err := validatedSkillID(id)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id = validatedID

	if h.skillsDir == "" {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "skills directory not configured")
	}

	resolved, installed := h.resolveInstalledSkill(id)
	if !installed {
		return h.legacyMarketInstall(ctx, id, userID)
	}
	id = resolved.ID
	targetDir := resolved.EntryDir

	rs := h.findRemoteSkill(ctx, id)
	if rs == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "skill not found in store")
	}

	h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
		"id": id, "percent": 0, "message": "Starting update...",
	})

	tempDir, err := os.MkdirTemp(h.skillsDir, ".skill-update-*")
	if err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	cleanupDir := tempDir
	defer func() {
		if cleanupDir != "" {
			_ = os.RemoveAll(cleanupDir)
		}
	}()

	manifest, err := h.materializeLegacyRemoteSkill(ctx, id, rs, tempDir, func(downloaded, total int) {
		if total <= 0 {
			return
		}
		pct := int(float64(downloaded) / float64(maxInt(total, 1)) * 100)
		h.publishEvent(userID, "skill.install.progress", map[string]interface{}{
			"id": id, "percent": pct, "message": "Downloading files...",
		})
	})
	if err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}

	backupDir := filepath.Join(filepath.Dir(targetDir), fmt.Sprintf(".%s.backup-%d", filepath.Base(targetDir), time.Now().UnixNano()))
	if err := os.Rename(targetDir, backupDir); err != nil {
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	if err := os.Rename(tempDir, targetDir); err != nil {
		_ = os.Rename(backupDir, targetDir)
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	cleanupDir = ""
	if err := h.syncInstalledSkillRegistration(manifest); err != nil {
		_ = os.RemoveAll(targetDir)
		_ = os.Rename(backupDir, targetDir)
		h.publishEvent(userID, "skill.install.error", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	_ = os.RemoveAll(backupDir)
	h.publishEvent(userID, "skill.install.complete", map[string]interface{}{"id": id})

	return legacyInstallResult(id, targetDir, rs), nil
}

func (h *SkillHandler) legacyMarketUninstall(id string) error {
	validatedID, err := validatedSkillID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id = validatedID

	if h.skillsDir == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "skills directory not configured")
	}
	skillDir := filepath.Join(h.skillsDir, id)
	if resolved, ok := h.resolveInstalledSkill(id); ok {
		id = resolved.ID
		skillDir = resolved.EntryDir
	} else if resolvedID, ok := h.resolveInstalledSkillID(id); ok {
		id = resolvedID
		skillDir = filepath.Join(h.skillsDir, id)
	}
	if info := h.registry.GetInfo(id); info != nil && info.Builtin {
		return echo.NewHTTPError(http.StatusForbidden, "cannot uninstall builtin skill")
	}
	_, dirErr := os.Stat(skillDir)
	inRegistry := h.registry.Get(id) != nil
	if os.IsNotExist(dirErr) && !inRegistry {
		return echo.NewHTTPError(http.StatusNotFound, "skill not installed")
	}
	if dirErr == nil {
		if err := os.RemoveAll(skillDir); err != nil {
			return err
		}
	}
	_ = h.registry.Unregister(id)
	if h.localScanner != nil {
		_ = h.localScanner.Scan()
	}
	return nil
}

func matchesFallbackMarketFilters(skill *RemoteSkill, query skillmarket.SearchQuery) bool {
	if skill == nil {
		return false
	}
	textQuery := strings.TrimSpace(strings.ToLower(query.Query))
	if textQuery != "" {
		joinedTags := strings.ToLower(strings.Join(skill.Tags, " "))
		if !strings.Contains(strings.ToLower(skill.Name), textQuery) &&
			!strings.Contains(strings.ToLower(skill.Description), textQuery) &&
			!strings.Contains(strings.ToLower(skill.Category), textQuery) &&
			!strings.Contains(joinedTags, textQuery) {
			return false
		}
	}
	categories := compactNonEmpty(append(append([]string{}, query.Categories...), query.Category))
	skillCategory := skillmarket.NormalizeMarketplaceCategory(skill.Category, skill.Description, skill.Tags)
	for i := range categories {
		categories[i] = skillmarket.NormalizeMarketplaceCategory(categories[i], "", nil)
	}
	if len(categories) > 0 && !containsFold(categories, skillCategory) {
		return false
	}
	sources := compactNonEmpty(query.Sources)
	if len(sources) > 0 && !containsFold(sources, skill.SourceID) && !containsFold(sources, skill.SourceName) && !containsFold(sources, skill.SourceGroup) {
		return false
	}
	if len(query.RiskBadges) > 0 && !containsFold(query.RiskBadges, skill.SecurityBadge) {
		return false
	}
	if len(query.InstallTypes) > 0 && !containsFold(query.InstallTypes, skill.InstallType) {
		return false
	}
	if len(query.ArtifactKinds) > 0 && !containsFold(query.ArtifactKinds, skill.ArtifactKind) {
		return false
	}
	if query.Installable != nil && skill.Installable != *query.Installable {
		return false
	}
	if query.Curated != nil && *query.Curated && skill.CuratedRank == 0 && strings.TrimSpace(skill.CuratedLabel) == "" && skill.SourceGroup != "featured" {
		return false
	}
	if query.OpenSourceOnly && skill.ArtifactKind != skillmarket.ArtifactKindOpenSource {
		return false
	}
	if query.HasVulnerabilities != nil && skill.HasVulnerabilities != *query.HasVulnerabilities {
		return false
	}
	if query.HasPromptInjection != nil && skill.HasPromptInjection != *query.HasPromptInjection {
		return false
	}
	if query.HasShellInjection != nil && skill.HasShellInjection != *query.HasShellInjection {
		return false
	}
	if query.HasDataExfiltration != nil && skill.HasDataExfiltration != *query.HasDataExfiltration {
		return false
	}
	return true
}

func sortFallbackMarketResults(results []fallbackMarketSearchResult, sortMode string) {
	switch sortMode {
	case "newest":
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Skill.Version > results[j].Skill.Version
		})
	case "most_used", "trending":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Skill.Downloads == results[j].Skill.Downloads {
				return results[i].Skill.Stars > results[j].Skill.Stars
			}
			return results[i].Skill.Downloads > results[j].Skill.Downloads
		})
	case "name":
		sort.SliceStable(results, func(i, j int) bool {
			return strings.ToLower(results[i].Skill.Name) < strings.ToLower(results[j].Skill.Name)
		})
	case "featured":
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Skill.CuratedRank > 0 || results[j].Skill.CuratedRank > 0 {
				if results[i].Skill.CuratedRank == 0 {
					return false
				}
				if results[j].Skill.CuratedRank == 0 {
					return true
				}
				return results[i].Skill.CuratedRank < results[j].Skill.CuratedRank
			}
			return results[i].Score > results[j].Score
		})
	default:
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}
}

func filterOptionsFromCounts(counts map[string]int) []skillmarket.FilterOption {
	keys := make([]string, 0, len(counts))
	for key, count := range counts {
		if strings.TrimSpace(key) == "" || count <= 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	options := make([]skillmarket.FilterOption, 0, len(keys))
	for _, key := range keys {
		options = append(options, skillmarket.FilterOption{
			Value: key,
			Label: key,
			Count: counts[key],
		})
	}
	return options
}

func compactNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func containsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
