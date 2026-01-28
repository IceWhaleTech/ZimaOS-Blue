package formfiller

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for form filler.
type Handler struct {
	store    *Store
	detector *Detector
}

// NewHandler creates a new form filler handler.
func NewHandler(store *Store) *Handler {
	return &Handler{
		store:    store,
		detector: NewDetector(store.GetPatterns()),
	}
}

// RegisterRoutes registers the form filler routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Template routes
	g.GET("/templates", h.ListTemplates)
	g.POST("/templates", h.CreateTemplate)
	g.GET("/templates/:id", h.GetTemplate)
	g.PUT("/templates/:id", h.UpdateTemplate)
	g.DELETE("/templates/:id", h.DeleteTemplate)

	// Pattern routes
	g.GET("/patterns", h.GetPatterns)
	g.PUT("/patterns", h.UpdatePatterns)

	// Site mapping routes
	g.GET("/sites/:domain", h.GetSiteMapping)
	g.PUT("/sites/:domain", h.SaveSiteMapping)

	// Detection route
	g.POST("/detect", h.DetectFields)

	// Config route
	g.GET("/config", h.GetConfig)
}

// ListTemplates returns all templates.
func (h *Handler) ListTemplates(c echo.Context) error {
	templates := h.store.ListTemplates()
	return c.JSON(http.StatusOK, templates)
}

// GetTemplate returns a template by ID.
func (h *Handler) GetTemplate(c echo.Context) error {
	id := c.Param("id")
	template, err := h.store.GetTemplate(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, template)
}

// CreateTemplate creates a new template.
func (h *Handler) CreateTemplate(c echo.Context) error {
	var req CreateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Name is required",
		})
	}

	template, err := h.store.CreateTemplate(&req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, template)
}

// UpdateTemplate updates an existing template.
func (h *Handler) UpdateTemplate(c echo.Context) error {
	id := c.Param("id")

	var req UpdateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	template, err := h.store.UpdateTemplate(id, &req)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, template)
}

// DeleteTemplate deletes a template.
func (h *Handler) DeleteTemplate(c echo.Context) error {
	id := c.Param("id")

	if err := h.store.DeleteTemplate(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// GetPatterns returns the field patterns.
func (h *Handler) GetPatterns(c echo.Context) error {
	patterns := h.store.GetPatterns()
	return c.JSON(http.StatusOK, patterns)
}

// UpdatePatterns updates the field patterns.
func (h *Handler) UpdatePatterns(c echo.Context) error {
	var patterns map[FieldType][]string
	if err := c.Bind(&patterns); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if err := h.store.UpdatePatterns(patterns); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Update detector with new patterns
	h.detector = NewDetector(h.store.GetPatterns())

	return c.JSON(http.StatusOK, h.store.GetPatterns())
}

// GetSiteMapping returns a site mapping by domain.
func (h *Handler) GetSiteMapping(c echo.Context) error {
	domain := c.Param("domain")

	mapping, err := h.store.GetSiteMapping(domain)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, mapping)
}

// SaveSiteMapping saves a site mapping.
func (h *Handler) SaveSiteMapping(c echo.Context) error {
	domain := c.Param("domain")

	var mapping SiteMapping
	if err := c.Bind(&mapping); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	mapping.Domain = domain

	if err := h.store.SaveSiteMapping(&mapping); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, mapping)
}

// DetectFields detects fields from HTML or field attributes.
func (h *Handler) DetectFields(c echo.Context) error {
	var req struct {
		Fields     []FieldAttributes `json:"fields"`
		TemplateID string            `json:"template_id,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	// Get template if specified
	var template *FillTemplate
	if req.TemplateID != "" {
		t, err := h.store.GetTemplate(req.TemplateID)
		if err == nil {
			template = t
		}
	} else {
		// Use default template
		template = h.store.GetDefaultTemplate()
	}

	// Detect fields
	detected := h.detector.DetectFields(req.Fields, template)

	return c.JSON(http.StatusOK, DetectResponse{
		Fields: detected,
		Count:  len(detected),
	})
}

// GetConfig returns the form filler configuration.
func (h *Handler) GetConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, DefaultConfig())
}
