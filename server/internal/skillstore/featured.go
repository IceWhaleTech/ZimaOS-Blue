package skillstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FeaturedSkill represents a curated skill in the featured list
type FeaturedSkill struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Version      string   `json:"version"`
	SourceURL    string   `json:"source_url"`
	Homepage     string   `json:"homepage"`
	Stars        int      `json:"stars"`
	FeaturedRank int      `json:"featured_rank"`
}

// FeaturedSkillsData represents the featured skills JSON file structure
type FeaturedSkillsData struct {
	Version   string           `json:"version"`
	UpdatedAt time.Time        `json:"updated_at"`
	Skills    []*FeaturedSkill `json:"skills"`
}

// FeaturedSkillsLoader loads and caches featured skills
type FeaturedSkillsLoader struct {
	dataPath string
	skills   []*FeaturedSkill
	mu       sync.RWMutex
	loaded   bool
}

// NewFeaturedSkillsLoader creates a new featured skills loader
func NewFeaturedSkillsLoader(dataPath string) *FeaturedSkillsLoader {
	return &FeaturedSkillsLoader{
		dataPath: dataPath,
		skills:   make([]*FeaturedSkill, 0),
	}
}

// Load loads featured skills from the JSON file
func (l *FeaturedSkillsLoader) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.dataPath)
	if err != nil {
		return fmt.Errorf("failed to read featured skills file: %w", err)
	}

	var featuredData FeaturedSkillsData
	if err := json.Unmarshal(data, &featuredData); err != nil {
		return fmt.Errorf("failed to parse featured skills: %w", err)
	}

	l.skills = featuredData.Skills
	l.loaded = true

	return nil
}

// GetAll returns all featured skills
func (l *FeaturedSkillsLoader) GetAll() []*FeaturedSkill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*FeaturedSkill, len(l.skills))
	copy(result, l.skills)
	return result
}

// Get returns a featured skill by ID
func (l *FeaturedSkillsLoader) Get(id string) *FeaturedSkill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, s := range l.skills {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Search searches featured skills by query
func (l *FeaturedSkillsLoader) Search(query string) []*FeaturedSkill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if query == "" {
		return l.GetAll()
	}

	query = strings.ToLower(query)
	var results []*FeaturedSkill

	for _, s := range l.skills {
		if matchesFeaturedSkill(s, query) {
			results = append(results, s)
		}
	}

	return results
}

// GetByCategory returns featured skills filtered by category
func (l *FeaturedSkillsLoader) GetByCategory(category string) []*FeaturedSkill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []*FeaturedSkill
	for _, s := range l.skills {
		if s.Category == category {
			results = append(results, s)
		}
	}
	return results
}

// GetCategories returns all unique categories
func (l *FeaturedSkillsLoader) GetCategories() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	categorySet := make(map[string]bool)
	for _, s := range l.skills {
		if s.Category != "" {
			categorySet[s.Category] = true
		}
	}

	categories := make([]string, 0, len(categorySet))
	for cat := range categorySet {
		categories = append(categories, cat)
	}
	return categories
}

// Count returns the number of featured skills
func (l *FeaturedSkillsLoader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.skills)
}

// IsLoaded returns whether the featured skills have been loaded
func (l *FeaturedSkillsLoader) IsLoaded() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loaded
}

// matchesFeaturedSkill checks if a skill matches the search query
func matchesFeaturedSkill(s *FeaturedSkill, query string) bool {
	// Empty query matches nothing (handled by caller)
	if query == "" {
		return false
	}

	// Normalize query to lowercase
	query = strings.ToLower(query)

	// Check name
	if strings.Contains(strings.ToLower(s.Name), query) {
		return true
	}

	// Check description
	if strings.Contains(strings.ToLower(s.Description), query) {
		return true
	}

	// Check ID
	if strings.Contains(strings.ToLower(s.ID), query) {
		return true
	}

	// Check author
	if strings.Contains(strings.ToLower(s.Author), query) {
		return true
	}

	// Check category
	if strings.Contains(strings.ToLower(s.Category), query) {
		return true
	}

	// Check tags
	for _, tag := range s.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}

	return false
}

// LocalSkill represents a skill discovered from local directory
type LocalSkill struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Version      string    `json:"version"`
	Author       string    `json:"author"`
	Category     string    `json:"category"`
	Tags         []string  `json:"tags"`
	FilePath     string    `json:"file_path"`
	Content      string    `json:"content"`
	DiscoveredAt time.Time `json:"discovered_at"`
	LastModified time.Time `json:"last_modified"`
}

// LocalSkillScanner scans local directories for skills
type LocalSkillScanner struct {
	basePath string
	skills   map[string]*LocalSkill
	mu       sync.RWMutex
}

// NewLocalSkillScanner creates a new local skill scanner
func NewLocalSkillScanner(basePath string) *LocalSkillScanner {
	return &LocalSkillScanner{
		basePath: basePath,
		skills:   make(map[string]*LocalSkill),
	}
}

// Scan scans the base directory for SKILL.md files
func (s *LocalSkillScanner) Scan() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing skills
	s.skills = make(map[string]*LocalSkill)

	// Check if base path exists
	if _, err := os.Stat(s.basePath); os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(s.basePath, 0755); err != nil {
			return fmt.Errorf("failed to create skills directory: %w", err)
		}
		return nil
	}

	// Walk the directory
	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Look for SKILL.md files
		if info.IsDir() || info.Name() != "SKILL.md" {
			return nil
		}

		skill, err := s.parseSkillFile(path)
		if err != nil {
			// Log error but continue scanning
			return nil
		}

		s.skills[skill.ID] = skill
		return nil
	})

	return err
}

// parseSkillFile parses a SKILL.md file
func (s *LocalSkillScanner) parseSkillFile(path string) (*LocalSkill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat skill file: %w", err)
	}

	// Parse YAML frontmatter
	skill := &LocalSkill{
		FilePath:     path,
		Content:      string(content),
		DiscoveredAt: time.Now(),
		LastModified: info.ModTime(),
	}

	// Extract frontmatter
	contentStr := string(content)
	if strings.HasPrefix(contentStr, "---") {
		parts := strings.SplitN(contentStr, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			skill.Content = strings.TrimSpace(parts[2])

			// Parse frontmatter lines
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				colonIdx := strings.Index(line, ":")
				if colonIdx == -1 {
					continue
				}

				key := strings.TrimSpace(line[:colonIdx])
				value := strings.TrimSpace(line[colonIdx+1:])

				switch key {
				case "name":
					skill.Name = value
					skill.ID = value // Use name as ID if not specified
				case "id":
					skill.ID = value
				case "description":
					skill.Description = value
				case "version":
					skill.Version = value
				case "author":
					skill.Author = value
				case "category":
					skill.Category = value
				case "tags":
					// Parse tags array [tag1, tag2]
					value = strings.Trim(value, "[]")
					for _, tag := range strings.Split(value, ",") {
						tag = strings.TrimSpace(tag)
						if tag != "" {
							skill.Tags = append(skill.Tags, tag)
						}
					}
				}
			}
		}
	}

	// Use directory name as ID if not set
	if skill.ID == "" {
		skill.ID = filepath.Base(filepath.Dir(path))
	}

	// Use ID as name if not set
	if skill.Name == "" {
		skill.Name = skill.ID
	}

	return skill, nil
}

// GetAll returns all discovered local skills
func (s *LocalSkillScanner) GetAll() []*LocalSkill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*LocalSkill, 0, len(s.skills))
	for _, skill := range s.skills {
		result = append(result, skill)
	}
	return result
}

// Get returns a local skill by ID
func (s *LocalSkillScanner) Get(id string) *LocalSkill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.skills[id]
}

// Count returns the number of discovered local skills
func (s *LocalSkillScanner) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.skills)
}
