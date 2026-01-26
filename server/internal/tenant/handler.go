package tenant

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for tenant management.
type Handler struct {
	service *Service
}

// NewHandler creates a new tenant handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the tenant routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	tenants := g.Group("/tenants")

	// Tenant CRUD
	tenants.POST("", h.CreateTenant)
	tenants.GET("", h.ListTenants)
	tenants.GET("/:id", h.GetTenant)
	tenants.PUT("/:id", h.UpdateTenant)
	tenants.DELETE("/:id", h.DeleteTenant)

	// Member management
	tenants.GET("/:id/members", h.ListMembers)
	tenants.PUT("/:id/members/:user_id", h.UpdateMember)
	tenants.DELETE("/:id/members/:user_id", h.RemoveMember)

	// Invitations
	tenants.POST("/:id/invitations", h.InviteMember)
	tenants.GET("/:id/invitations", h.ListInvitations)
	tenants.DELETE("/:id/invitations/:invitation_id", h.CancelInvitation)

	// Accept invitation (public route with token)
	g.POST("/invitations/accept", h.AcceptInvitation)

	// Settings and limits
	tenants.GET("/:id/settings", h.GetSettings)
	tenants.PUT("/:id/settings", h.UpdateSettings)
	tenants.GET("/:id/limits", h.GetLimits)
	tenants.PUT("/:id/limits", h.UpdateLimits)

	// Ownership transfer
	tenants.POST("/:id/transfer", h.TransferOwnership)
}

// CreateTenant handles tenant creation.
func (h *Handler) CreateTenant(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req CreateTenantRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	tenant, err := h.service.Create(c.Request().Context(), &req, userID)
	if err != nil {
		switch err {
		case ErrSlugExists:
			return echo.NewHTTPError(http.StatusConflict, "slug already exists")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create tenant")
		}
	}

	return c.JSON(http.StatusCreated, tenant)
}

// GetTenant handles getting a tenant by ID.
func (h *Handler) GetTenant(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user is a member
	isMember, err := h.service.IsMember(c.Request().Context(), tenantID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
	}
	if !isMember {
		return echo.NewHTTPError(http.StatusForbidden, "not a member of this tenant")
	}

	tenant, err := h.service.GetByID(c.Request().Context(), tenantID)
	if err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tenant")
	}

	return c.JSON(http.StatusOK, tenant)
}

// UpdateTenant handles updating a tenant.
func (h *Handler) UpdateTenant(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user has admin permission
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
	}

	var req UpdateTenantRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	tenant, err := h.service.Update(c.Request().Context(), tenantID, &req)
	if err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update tenant")
	}

	return c.JSON(http.StatusOK, tenant)
}

// DeleteTenant handles deleting a tenant.
func (h *Handler) DeleteTenant(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Only owner can delete tenant
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleOwner)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "only owner can delete tenant")
	}

	if err := h.service.Delete(c.Request().Context(), tenantID); err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete tenant")
	}

	return c.NoContent(http.StatusNoContent)
}

// ListTenants handles listing tenants for the current user.
func (h *Handler) ListTenants(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenants, err := h.service.GetUserTenants(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tenants")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"total":   len(tenants),
	})
}

// ListMembers handles listing members of a tenant.
func (h *Handler) ListMembers(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user is a member
	isMember, err := h.service.IsMember(c.Request().Context(), tenantID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
	}
	if !isMember {
		return echo.NewHTTPError(http.StatusForbidden, "not a member of this tenant")
	}

	query := &ListMembersQuery{
		Page:     1,
		PageSize: 20,
	}

	if page := c.QueryParam("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			query.Page = p
		}
	}

	if pageSize := c.QueryParam("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			query.PageSize = ps
		}
	}

	if role := c.QueryParam("role"); role != "" {
		r := MemberRole(role)
		query.Role = &r
	}

	result, err := h.service.ListMembers(c.Request().Context(), tenantID, query)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list members")
	}

	return c.JSON(http.StatusOK, result)
}

// UpdateMember handles updating a member's role.
func (h *Handler) UpdateMember(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	memberUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	// Check if user has admin permission
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
	}

	var req UpdateMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.UpdateMemberRole(c.Request().Context(), tenantID, memberUserID, req.Role); err != nil {
		switch err {
		case ErrMemberNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "member not found")
		case ErrCannotChangeOwnerRole:
			return echo.NewHTTPError(http.StatusForbidden, "cannot change owner's role")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to update member")
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// RemoveMember handles removing a member from a tenant.
func (h *Handler) RemoveMember(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	memberUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	// Check if user has admin permission (or is removing themselves)
	if memberUserID != userID {
		hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
		}
		if !hasPermission {
			return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
		}
	}

	if err := h.service.RemoveMember(c.Request().Context(), tenantID, memberUserID); err != nil {
		switch err {
		case ErrMemberNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "member not found")
		case ErrCannotRemoveOwner:
			return echo.NewHTTPError(http.StatusForbidden, "cannot remove tenant owner")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove member")
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// InviteMember handles inviting a new member.
func (h *Handler) InviteMember(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user has admin permission
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
	}

	var req InviteMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	invitation, err := h.service.InviteMember(c.Request().Context(), tenantID, &req, userID)
	if err != nil {
		switch err {
		case ErrTenantNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		case ErrLimitExceeded:
			return echo.NewHTTPError(http.StatusForbidden, "member limit exceeded")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create invitation")
		}
	}

	return c.JSON(http.StatusCreated, invitation)
}

// ListInvitations handles listing pending invitations.
func (h *Handler) ListInvitations(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user has admin permission
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
	}

	invitations, err := h.service.ListPendingInvitations(c.Request().Context(), tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list invitations")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"invitations": invitations,
		"total":       len(invitations),
	})
}

// CancelInvitation handles canceling an invitation.
func (h *Handler) CancelInvitation(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	invitationID, err := uuid.Parse(c.Param("invitation_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid invitation ID")
	}

	// Check if user has admin permission
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
	}

	if err := h.service.CancelInvitation(c.Request().Context(), invitationID); err != nil {
		if err == ErrInvitationNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "invitation not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to cancel invitation")
	}

	return c.NoContent(http.StatusNoContent)
}

// AcceptInvitationRequest represents a request to accept an invitation.
type AcceptInvitationRequest struct {
	Token string `json:"token" validate:"required"`
}

// AcceptInvitation handles accepting an invitation.
func (h *Handler) AcceptInvitation(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req AcceptInvitationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	tenant, err := h.service.AcceptInvitation(c.Request().Context(), req.Token, userID)
	if err != nil {
		switch err {
		case ErrInvitationNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "invitation not found")
		case ErrInvitationExpired:
			return echo.NewHTTPError(http.StatusGone, "invitation has expired")
		case ErrInvitationAlreadyAccepted:
			return echo.NewHTTPError(http.StatusConflict, "invitation already accepted")
		case ErrMemberExists:
			return echo.NewHTTPError(http.StatusConflict, "already a member of this tenant")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to accept invitation")
		}
	}

	return c.JSON(http.StatusOK, tenant)
}

// GetSettings handles getting tenant settings.
func (h *Handler) GetSettings(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user is a member
	isMember, err := h.service.IsMember(c.Request().Context(), tenantID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
	}
	if !isMember {
		return echo.NewHTTPError(http.StatusForbidden, "not a member of this tenant")
	}

	settings, err := h.service.GetTenantSettings(c.Request().Context(), tenantID)
	if err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get settings")
	}

	return c.JSON(http.StatusOK, settings)
}

// UpdateSettings handles updating tenant settings.
func (h *Handler) UpdateSettings(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user has admin permission
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
	}

	var settings TenantSettings
	if err := c.Bind(&settings); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.UpdateTenantSettings(c.Request().Context(), tenantID, &settings); err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update settings")
	}

	return c.NoContent(http.StatusNoContent)
}

// GetLimits handles getting tenant limits.
func (h *Handler) GetLimits(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Check if user is a member
	isMember, err := h.service.IsMember(c.Request().Context(), tenantID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
	}
	if !isMember {
		return echo.NewHTTPError(http.StatusForbidden, "not a member of this tenant")
	}

	limits, err := h.service.GetTenantLimits(c.Request().Context(), tenantID)
	if err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get limits")
	}

	return c.JSON(http.StatusOK, limits)
}

// UpdateLimits handles updating tenant limits (admin only).
func (h *Handler) UpdateLimits(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	// Only owner can update limits
	hasPermission, err := h.service.HasPermission(c.Request().Context(), tenantID, userID, MemberRoleOwner)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permission")
	}
	if !hasPermission {
		return echo.NewHTTPError(http.StatusForbidden, "only owner can update limits")
	}

	var limits TenantLimits
	if err := c.Bind(&limits); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.UpdateTenantLimits(c.Request().Context(), tenantID, &limits); err != nil {
		if err == ErrTenantNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update limits")
	}

	return c.NoContent(http.StatusNoContent)
}

// TransferOwnershipRequest represents a request to transfer ownership.
type TransferOwnershipRequest struct {
	NewOwnerID uuid.UUID `json:"new_owner_id" validate:"required"`
}

// TransferOwnership handles transferring tenant ownership.
func (h *Handler) TransferOwnership(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
	}

	var req TransferOwnershipRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.TransferOwnership(c.Request().Context(), tenantID, userID, req.NewOwnerID); err != nil {
		switch err {
		case ErrTenantNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
		case ErrMemberNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "new owner is not a member")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to transfer ownership")
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// Helper function to get user ID from context
func getUserIDFromContext(c echo.Context) uuid.UUID {
	if id, ok := c.Get("user_id").(uuid.UUID); ok {
		return id
	}
	if idStr, ok := c.Get("user_id").(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			return id
		}
	}
	return uuid.Nil
}
