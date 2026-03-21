package selfreflect

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	if h == nil || h.service == nil || g == nil {
		return
	}
	g.GET("/self-reflect/proposals", h.ListProposals)
	g.GET("/self-reflect/proposals/:id", h.GetProposal)
	g.GET("/self-reflect/proposals/:id/patch", h.GetPatchPreview)
	g.POST("/self-reflect/proposals/:id/approve", h.ApproveProposal)
	g.POST("/self-reflect/proposals/:id/reject", h.RejectProposal)
}

func (h *Handler) ListProposals(c echo.Context) error {
	filter := ProposalFilter{
		OwnerUserID: proposalUserID(c),
		SourceKind:  strings.TrimSpace(c.QueryParam("source_kind")),
		SourceID:    strings.TrimSpace(c.QueryParam("source_id")),
		Limit:       50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if rawStatuses := c.QueryParams()["status"]; len(rawStatuses) > 0 {
		filter.Statuses = parseProposalStatuses(rawStatuses...)
	} else if raw := c.QueryParam("status"); strings.TrimSpace(raw) != "" {
		filter.Statuses = parseProposalStatuses(raw)
	}
	proposals, err := h.service.ListProposals(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, proposals)
}

func (h *Handler) GetProposal(c echo.Context) error {
	proposal, err := h.scopedProposal(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "proposal not found"})
	}
	return c.JSON(http.StatusOK, proposal)
}

func (h *Handler) GetPatchPreview(c echo.Context) error {
	proposal, err := h.scopedProposal(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "proposal not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":            proposal.ID,
		"target_file":   proposal.TargetFile,
		"patch_preview": proposal.PatchPreview,
	})
}

func (h *Handler) ApproveProposal(c echo.Context) error {
	return h.reviewProposal(c, ProposalStatusApproved)
}

func (h *Handler) RejectProposal(c echo.Context) error {
	return h.reviewProposal(c, ProposalStatusRejected)
}

func (h *Handler) reviewProposal(c echo.Context, status ProposalStatus) error {
	proposal, err := h.scopedProposal(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "proposal not found"})
	}
	var body struct {
		ReviewNote string `json:"review_note"`
	}
	_ = c.Bind(&body)
	updated, err := h.service.ReviewProposal(c.Request().Context(), proposal.ID, status, body.ReviewNote)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, updated)
}

func (h *Handler) scopedProposal(c echo.Context) (*Proposal, error) {
	proposal, err := h.service.GetProposal(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := proposalUserID(c); userID != "" && proposal.OwnerUserID != "" && proposal.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return proposal, nil
}

func proposalUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return strings.TrimSpace(claims.UserID)
	}
	return ""
}

func parseProposalStatuses(values ...string) []ProposalStatus {
	out := make([]ProposalStatus, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			status := ProposalStatus(strings.ToLower(strings.TrimSpace(item)))
			switch status {
			case ProposalStatusPending, ProposalStatusApproved, ProposalStatusRejected:
				out = append(out, status)
			}
		}
	}
	return out
}
