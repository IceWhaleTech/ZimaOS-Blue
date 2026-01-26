package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLiteRepository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// InitSchema creates the necessary tables.
func (r *SQLiteRepository) InitSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			settings TEXT,
			limits TEXT,
			owner_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			deleted_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_owner_id ON tenants(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status)`,

		`CREATE TABLE IF NOT EXISTS tenant_members (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at DATETIME NOT NULL,
			invited_by TEXT,
			UNIQUE(tenant_id, user_id),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_members_tenant_id ON tenant_members(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_members_user_id ON tenant_members(user_id)`,

		`CREATE TABLE IF NOT EXISTS tenant_invitations (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			email TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			token TEXT NOT NULL UNIQUE,
			invited_by TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			accepted_at DATETIME,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_invitations_tenant_id ON tenant_invitations(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_invitations_token ON tenant_invitations(token)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_invitations_email ON tenant_invitations(email)`,
	}

	for _, query := range queries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

// Create creates a new tenant.
func (r *SQLiteRepository) Create(ctx context.Context, tenant *Tenant) error {
	query := `INSERT INTO tenants (id, name, slug, description, status, settings, limits, owner_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		tenant.ID.String(),
		tenant.Name,
		tenant.Slug,
		tenant.Description,
		tenant.Status,
		tenant.Settings,
		tenant.Limits,
		tenant.OwnerID.String(),
		tenant.CreatedAt,
		tenant.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrSlugExists
		}
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	return nil
}

// GetByID retrieves a tenant by ID.
func (r *SQLiteRepository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	query := `SELECT id, name, slug, description, status, settings, limits, owner_id, created_at, updated_at, deleted_at
		FROM tenants WHERE id = ? AND deleted_at IS NULL`

	tenant := &Tenant{}
	var idStr, ownerIDStr string
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&tenant.Name,
		&tenant.Slug,
		&tenant.Description,
		&tenant.Status,
		&tenant.Settings,
		&tenant.Limits,
		&ownerIDStr,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
		&tenant.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	tenant.ID, _ = uuid.Parse(idStr)
	tenant.OwnerID, _ = uuid.Parse(ownerIDStr)

	return tenant, nil
}

// GetBySlug retrieves a tenant by slug.
func (r *SQLiteRepository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	query := `SELECT id, name, slug, description, status, settings, limits, owner_id, created_at, updated_at, deleted_at
		FROM tenants WHERE slug = ? AND deleted_at IS NULL`

	tenant := &Tenant{}
	var idStr, ownerIDStr string
	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&idStr,
		&tenant.Name,
		&tenant.Slug,
		&tenant.Description,
		&tenant.Status,
		&tenant.Settings,
		&tenant.Limits,
		&ownerIDStr,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
		&tenant.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTenantNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	tenant.ID, _ = uuid.Parse(idStr)
	tenant.OwnerID, _ = uuid.Parse(ownerIDStr)

	return tenant, nil
}

// Update updates a tenant.
func (r *SQLiteRepository) Update(ctx context.Context, tenant *Tenant) error {
	query := `UPDATE tenants SET name = ?, description = ?, status = ?, settings = ?, limits = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query,
		tenant.Name,
		tenant.Description,
		tenant.Status,
		tenant.Settings,
		tenant.Limits,
		time.Now().UTC(),
		tenant.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrTenantNotFound
	}

	return nil
}

// Delete soft-deletes a tenant.
func (r *SQLiteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tenants SET deleted_at = ?, status = ? WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), StatusDeleted, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrTenantNotFound
	}

	return nil
}

// List retrieves tenants with pagination and filtering.
func (r *SQLiteRepository) List(ctx context.Context, query *ListTenantsQuery) (*ListTenantsResponse, error) {
	// Set defaults
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	// Build query
	baseQuery := `FROM tenants WHERE deleted_at IS NULL`
	args := []interface{}{}

	if query.Search != "" {
		baseQuery += ` AND (name LIKE ? OR slug LIKE ?)`
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if query.Status != nil {
		baseQuery += ` AND status = ?`
		args = append(args, *query.Status)
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) ` + baseQuery
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count tenants: %w", err)
	}

	// Build order clause
	orderBy := "created_at"
	if query.SortBy != "" {
		switch query.SortBy {
		case "name", "slug", "created_at", "updated_at":
			orderBy = query.SortBy
		}
	}
	orderDir := "DESC"
	if query.SortDir == "asc" {
		orderDir = "ASC"
	}

	// Fetch tenants
	selectQuery := `SELECT id, name, slug, description, status, settings, limits, owner_id, created_at, updated_at, deleted_at ` +
		baseQuery + fmt.Sprintf(` ORDER BY %s %s LIMIT ? OFFSET ?`, orderBy, orderDir)

	offset := (query.Page - 1) * query.PageSize
	args = append(args, query.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	tenants := make([]*Tenant, 0)
	for rows.Next() {
		tenant := &Tenant{}
		var idStr, ownerIDStr string
		if err := rows.Scan(
			&idStr,
			&tenant.Name,
			&tenant.Slug,
			&tenant.Description,
			&tenant.Status,
			&tenant.Settings,
			&tenant.Limits,
			&ownerIDStr,
			&tenant.CreatedAt,
			&tenant.UpdatedAt,
			&tenant.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}
		tenant.ID, _ = uuid.Parse(idStr)
		tenant.OwnerID, _ = uuid.Parse(ownerIDStr)
		tenants = append(tenants, tenant)
	}

	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}

	return &ListTenantsResponse{
		Tenants:    tenants,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ExistsBySlug checks if a slug exists.
func (r *SQLiteRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM tenants WHERE slug = ? AND deleted_at IS NULL)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check slug existence: %w", err)
	}
	return exists, nil
}

// GetUserTenants retrieves all tenants a user belongs to.
func (r *SQLiteRepository) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*Tenant, error) {
	query := `SELECT t.id, t.name, t.slug, t.description, t.status, t.settings, t.limits, t.owner_id, t.created_at, t.updated_at, t.deleted_at
		FROM tenants t
		INNER JOIN tenant_members tm ON t.id = tm.tenant_id
		WHERE tm.user_id = ? AND t.deleted_at IS NULL
		ORDER BY t.name ASC`

	rows, err := r.db.QueryContext(ctx, query, userID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get user tenants: %w", err)
	}
	defer rows.Close()

	tenants := make([]*Tenant, 0)
	for rows.Next() {
		tenant := &Tenant{}
		var idStr, ownerIDStr string
		if err := rows.Scan(
			&idStr,
			&tenant.Name,
			&tenant.Slug,
			&tenant.Description,
			&tenant.Status,
			&tenant.Settings,
			&tenant.Limits,
			&ownerIDStr,
			&tenant.CreatedAt,
			&tenant.UpdatedAt,
			&tenant.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}
		tenant.ID, _ = uuid.Parse(idStr)
		tenant.OwnerID, _ = uuid.Parse(ownerIDStr)
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}

// AddMember adds a member to a tenant.
func (r *SQLiteRepository) AddMember(ctx context.Context, member *TenantMember) error {
	query := `INSERT INTO tenant_members (id, tenant_id, user_id, role, joined_at, invited_by)
		VALUES (?, ?, ?, ?, ?, ?)`

	var invitedBy *string
	if member.InvitedBy != nil {
		s := member.InvitedBy.String()
		invitedBy = &s
	}

	_, err := r.db.ExecContext(ctx, query,
		member.ID.String(),
		member.TenantID.String(),
		member.UserID.String(),
		member.Role,
		member.JoinedAt,
		invitedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrMemberExists
		}
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

// GetMember retrieves a member by tenant and user ID.
func (r *SQLiteRepository) GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*TenantMember, error) {
	query := `SELECT id, tenant_id, user_id, role, joined_at, invited_by
		FROM tenant_members WHERE tenant_id = ? AND user_id = ?`

	member := &TenantMember{}
	var idStr, tenantIDStr, userIDStr string
	var invitedByStr *string
	err := r.db.QueryRowContext(ctx, query, tenantID.String(), userID.String()).Scan(
		&idStr,
		&tenantIDStr,
		&userIDStr,
		&member.Role,
		&member.JoinedAt,
		&invitedByStr,
	)
	if err == sql.ErrNoRows {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	member.ID, _ = uuid.Parse(idStr)
	member.TenantID, _ = uuid.Parse(tenantIDStr)
	member.UserID, _ = uuid.Parse(userIDStr)
	if invitedByStr != nil {
		invitedBy, _ := uuid.Parse(*invitedByStr)
		member.InvitedBy = &invitedBy
	}

	return member, nil
}

// UpdateMember updates a member's role.
func (r *SQLiteRepository) UpdateMember(ctx context.Context, member *TenantMember) error {
	query := `UPDATE tenant_members SET role = ? WHERE tenant_id = ? AND user_id = ?`

	result, err := r.db.ExecContext(ctx, query, member.Role, member.TenantID.String(), member.UserID.String())
	if err != nil {
		return fmt.Errorf("failed to update member: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrMemberNotFound
	}

	return nil
}

// RemoveMember removes a member from a tenant.
func (r *SQLiteRepository) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	query := `DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?`

	result, err := r.db.ExecContext(ctx, query, tenantID.String(), userID.String())
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrMemberNotFound
	}

	return nil
}

// ListMembers retrieves members of a tenant with pagination.
func (r *SQLiteRepository) ListMembers(ctx context.Context, tenantID uuid.UUID, query *ListMembersQuery) (*ListMembersResponse, error) {
	// Set defaults
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	// Build query
	baseQuery := `FROM tenant_members tm
		INNER JOIN users u ON tm.user_id = u.id
		WHERE tm.tenant_id = ? AND u.deleted_at IS NULL`
	args := []interface{}{tenantID.String()}

	if query.Role != nil {
		baseQuery += ` AND tm.role = ?`
		args = append(args, *query.Role)
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) ` + baseQuery
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count members: %w", err)
	}

	// Fetch members
	selectQuery := `SELECT tm.id, tm.tenant_id, tm.user_id, tm.role, tm.joined_at, tm.invited_by, u.username, u.email ` +
		baseQuery + ` ORDER BY tm.joined_at DESC LIMIT ? OFFSET ?`

	offset := (query.Page - 1) * query.PageSize
	args = append(args, query.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	defer rows.Close()

	members := make([]*TenantMemberWithUser, 0)
	for rows.Next() {
		member := &TenantMemberWithUser{}
		var idStr, tenantIDStr, userIDStr string
		var invitedByStr *string
		if err := rows.Scan(
			&idStr,
			&tenantIDStr,
			&userIDStr,
			&member.Role,
			&member.JoinedAt,
			&invitedByStr,
			&member.Username,
			&member.Email,
		); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		member.ID, _ = uuid.Parse(idStr)
		member.TenantID, _ = uuid.Parse(tenantIDStr)
		member.UserID, _ = uuid.Parse(userIDStr)
		if invitedByStr != nil {
			invitedBy, _ := uuid.Parse(*invitedByStr)
			member.InvitedBy = &invitedBy
		}
		members = append(members, member)
	}

	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}

	return &ListMembersResponse{
		Members:    members,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

// CountMembers returns the number of members in a tenant.
func (r *SQLiteRepository) CountMembers(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM tenant_members WHERE tenant_id = ?`
	var count int64
	if err := r.db.QueryRowContext(ctx, query, tenantID.String()).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count members: %w", err)
	}
	return count, nil
}

// IsMember checks if a user is a member of a tenant.
func (r *SQLiteRepository) IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM tenant_members WHERE tenant_id = ? AND user_id = ?)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, tenantID.String(), userID.String()).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}
	return exists, nil
}

// CreateInvitation creates a new invitation.
func (r *SQLiteRepository) CreateInvitation(ctx context.Context, invitation *TenantInvitation) error {
	query := `INSERT INTO tenant_invitations (id, tenant_id, email, role, token, invited_by, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		invitation.ID.String(),
		invitation.TenantID.String(),
		invitation.Email,
		invitation.Role,
		invitation.Token,
		invitation.InvitedBy.String(),
		invitation.ExpiresAt,
		invitation.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}

	return nil
}

// GetInvitationByID retrieves an invitation by ID.
func (r *SQLiteRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*TenantInvitation, error) {
	query := `SELECT id, tenant_id, email, role, token, invited_by, expires_at, accepted_at, created_at
		FROM tenant_invitations WHERE id = ?`

	invitation := &TenantInvitation{}
	var idStr, tenantIDStr, invitedByStr string
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&tenantIDStr,
		&invitation.Email,
		&invitation.Role,
		&invitation.Token,
		&invitedByStr,
		&invitation.ExpiresAt,
		&invitation.AcceptedAt,
		&invitation.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}

	invitation.ID, _ = uuid.Parse(idStr)
	invitation.TenantID, _ = uuid.Parse(tenantIDStr)
	invitation.InvitedBy, _ = uuid.Parse(invitedByStr)

	return invitation, nil
}

// GetInvitationByToken retrieves an invitation by token.
func (r *SQLiteRepository) GetInvitationByToken(ctx context.Context, token string) (*TenantInvitation, error) {
	query := `SELECT id, tenant_id, email, role, token, invited_by, expires_at, accepted_at, created_at
		FROM tenant_invitations WHERE token = ?`

	invitation := &TenantInvitation{}
	var idStr, tenantIDStr, invitedByStr string
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&idStr,
		&tenantIDStr,
		&invitation.Email,
		&invitation.Role,
		&invitation.Token,
		&invitedByStr,
		&invitation.ExpiresAt,
		&invitation.AcceptedAt,
		&invitation.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}

	invitation.ID, _ = uuid.Parse(idStr)
	invitation.TenantID, _ = uuid.Parse(tenantIDStr)
	invitation.InvitedBy, _ = uuid.Parse(invitedByStr)

	return invitation, nil
}

// GetPendingInvitationByEmail retrieves a pending invitation by email for a tenant.
func (r *SQLiteRepository) GetPendingInvitationByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*TenantInvitation, error) {
	query := `SELECT id, tenant_id, email, role, token, invited_by, expires_at, accepted_at, created_at
		FROM tenant_invitations WHERE tenant_id = ? AND email = ? AND accepted_at IS NULL AND expires_at > ?`

	invitation := &TenantInvitation{}
	var idStr, tenantIDStr, invitedByStr string
	err := r.db.QueryRowContext(ctx, query, tenantID.String(), email, time.Now().UTC()).Scan(
		&idStr,
		&tenantIDStr,
		&invitation.Email,
		&invitation.Role,
		&invitation.Token,
		&invitedByStr,
		&invitation.ExpiresAt,
		&invitation.AcceptedAt,
		&invitation.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}

	invitation.ID, _ = uuid.Parse(idStr)
	invitation.TenantID, _ = uuid.Parse(tenantIDStr)
	invitation.InvitedBy, _ = uuid.Parse(invitedByStr)

	return invitation, nil
}

// AcceptInvitation marks an invitation as accepted.
func (r *SQLiteRepository) AcceptInvitation(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tenant_invitations SET accepted_at = ? WHERE id = ? AND accepted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id.String())
	if err != nil {
		return fmt.Errorf("failed to accept invitation: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvitationNotFound
	}

	return nil
}

// DeleteInvitation deletes an invitation.
func (r *SQLiteRepository) DeleteInvitation(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tenant_invitations WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete invitation: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvitationNotFound
	}

	return nil
}

// ListPendingInvitations retrieves pending invitations for a tenant.
func (r *SQLiteRepository) ListPendingInvitations(ctx context.Context, tenantID uuid.UUID) ([]*TenantInvitation, error) {
	query := `SELECT id, tenant_id, email, role, token, invited_by, expires_at, accepted_at, created_at
		FROM tenant_invitations WHERE tenant_id = ? AND accepted_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, tenantID.String(), time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to list invitations: %w", err)
	}
	defer rows.Close()

	invitations := make([]*TenantInvitation, 0)
	for rows.Next() {
		invitation := &TenantInvitation{}
		var idStr, tenantIDStr, invitedByStr string
		if err := rows.Scan(
			&idStr,
			&tenantIDStr,
			&invitation.Email,
			&invitation.Role,
			&invitation.Token,
			&invitedByStr,
			&invitation.ExpiresAt,
			&invitation.AcceptedAt,
			&invitation.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan invitation: %w", err)
		}
		invitation.ID, _ = uuid.Parse(idStr)
		invitation.TenantID, _ = uuid.Parse(tenantIDStr)
		invitation.InvitedBy, _ = uuid.Parse(invitedByStr)
		invitations = append(invitations, invitation)
	}

	return invitations, nil
}

// CleanupExpiredInvitations removes expired invitations.
func (r *SQLiteRepository) CleanupExpiredInvitations(ctx context.Context) error {
	query := `DELETE FROM tenant_invitations WHERE expires_at < ? AND accepted_at IS NULL`

	_, err := r.db.ExecContext(ctx, query, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to cleanup invitations: %w", err)
	}

	return nil
}
