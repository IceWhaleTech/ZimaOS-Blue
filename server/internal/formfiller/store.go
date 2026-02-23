package formfiller

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Store handles persistence for form filler data.
type Store struct {
	dataDir   string
	mu        sync.RWMutex
	templates map[string]*FillTemplate
	patterns  *FieldPatterns
	sites     map[string]*SiteMapping
}

// NewStore creates a new form filler store.
func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		dataDir:   dataDir,
		templates: make(map[string]*FillTemplate),
		patterns:  DefaultPatterns(),
		sites:     make(map[string]*SiteMapping),
	}

	// Create data directories
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "sites"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Load existing data
	if err := s.loadTemplates(); err != nil {
		// Not an error if file doesn't exist
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load templates: %w", err)
		}
	}

	if err := s.loadPatterns(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load patterns: %w", err)
		}
	}

	// Create default template if none exists
	if len(s.templates) == 0 {
		defaultTemplate := &FillTemplate{
			ID:        uuid.New().String(),
			Name:      "Personal",
			IsDefault: true,
			Fields:    make(map[string]string),
			CreatedAt: timeutil.NowTime(),
			UpdatedAt: timeutil.NowTime(),
		}
		s.templates[defaultTemplate.ID] = defaultTemplate
		_ = s.saveTemplates()
	}

	return s, nil
}

// templatesPath returns the path to the templates file.
func (s *Store) templatesPath() string {
	return filepath.Join(s.dataDir, "templates.json")
}

// patternsPath returns the path to the patterns file.
func (s *Store) patternsPath() string {
	return filepath.Join(s.dataDir, "patterns.json")
}

// sitePath returns the path to a site mapping file.
func (s *Store) sitePath(domain string) string {
	return filepath.Join(s.dataDir, "sites", domain+".json")
}

// loadTemplates loads templates from disk.
func (s *Store) loadTemplates() error {
	data, err := os.ReadFile(s.templatesPath())
	if err != nil {
		return err
	}

	var templates []*FillTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		return err
	}

	s.templates = make(map[string]*FillTemplate)
	for _, t := range templates {
		s.templates[t.ID] = t
	}
	return nil
}

// saveTemplates saves templates to disk.
func (s *Store) saveTemplates() error {
	templates := make([]*FillTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		templates = append(templates, t)
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.templatesPath(), data, 0644)
}

// loadPatterns loads patterns from disk.
func (s *Store) loadPatterns() error {
	data, err := os.ReadFile(s.patternsPath())
	if err != nil {
		return err
	}

	var patterns FieldPatterns
	if err := json.Unmarshal(data, &patterns); err != nil {
		return err
	}

	s.patterns = &patterns
	return nil
}

// savePatterns saves patterns to disk.
func (s *Store) savePatterns() error {
	data, err := json.MarshalIndent(s.patterns, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.patternsPath(), data, 0644)
}

// loadSiteMapping loads a site mapping from disk.
func (s *Store) loadSiteMapping(domain string) (*SiteMapping, error) {
	data, err := os.ReadFile(s.sitePath(domain))
	if err != nil {
		return nil, err
	}

	var mapping SiteMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}

	return &mapping, nil
}

// saveSiteMapping saves a site mapping to disk.
func (s *Store) saveSiteMapping(mapping *SiteMapping) error {
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.sitePath(mapping.Domain), data, 0644)
}

// ListTemplates returns all templates.
func (s *Store) ListTemplates() []*FillTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	templates := make([]*FillTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		templates = append(templates, t)
	}
	return templates
}

// GetTemplate returns a template by ID.
func (s *Store) GetTemplate(id string) (*FillTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return t, nil
}

// GetDefaultTemplate returns the default template.
func (s *Store) GetDefaultTemplate() *FillTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.templates {
		if t.IsDefault {
			return t
		}
	}
	return nil
}

// CreateTemplate creates a new template.
func (s *Store) CreateTemplate(req *CreateTemplateRequest) (*FillTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If this is set as default, unset other defaults
	if req.IsDefault {
		for _, t := range s.templates {
			t.IsDefault = false
		}
	}

	template := &FillTemplate{
		ID:        uuid.New().String(),
		Name:      req.Name,
		IsDefault: req.IsDefault,
		Fields:    req.Fields,
		CreatedAt: timeutil.NowTime(),
		UpdatedAt: timeutil.NowTime(),
	}

	if template.Fields == nil {
		template.Fields = make(map[string]string)
	}

	s.templates[template.ID] = template

	if err := s.saveTemplates(); err != nil {
		delete(s.templates, template.ID)
		return nil, err
	}

	return template, nil
}

// UpdateTemplate updates an existing template.
func (s *Store) UpdateTemplate(id string, req *UpdateTemplateRequest) (*FillTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}

	if req.Name != "" {
		t.Name = req.Name
	}
	if req.IsDefault != nil {
		// If setting as default, unset other defaults
		if *req.IsDefault {
			for _, other := range s.templates {
				other.IsDefault = false
			}
		}
		t.IsDefault = *req.IsDefault
	}
	if req.Fields != nil {
		t.Fields = req.Fields
	}
	t.UpdatedAt = timeutil.NowTime()

	if err := s.saveTemplates(); err != nil {
		return nil, err
	}

	return t, nil
}

// DeleteTemplate deletes a template.
func (s *Store) DeleteTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.templates[id]; !ok {
		return fmt.Errorf("template not found: %s", id)
	}

	delete(s.templates, id)

	return s.saveTemplates()
}

// GetPatterns returns the field patterns.
func (s *Store) GetPatterns() *FieldPatterns {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.patterns
}

// UpdatePatterns updates the field patterns.
func (s *Store) UpdatePatterns(patterns map[FieldType][]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.patterns.Patterns = patterns
	s.patterns.UpdatedAt = timeutil.NowTime()

	return s.savePatterns()
}

// GetSiteMapping returns a site mapping by domain.
func (s *Store) GetSiteMapping(domain string) (*SiteMapping, error) {
	s.mu.RLock()
	mapping, ok := s.sites[domain]
	s.mu.RUnlock()

	if ok {
		return mapping, nil
	}

	// Try to load from disk
	mapping, err := s.loadSiteMapping(domain)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("site mapping not found: %s", domain)
		}
		return nil, err
	}

	s.mu.Lock()
	s.sites[domain] = mapping
	s.mu.Unlock()

	return mapping, nil
}

// SaveSiteMapping saves a site mapping.
func (s *Store) SaveSiteMapping(mapping *SiteMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mapping.LastUsed = timeutil.NowTime()
	s.sites[mapping.Domain] = mapping

	return s.saveSiteMapping(mapping)
}
