package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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

// Table helpers

func (r *SQLiteRepository) tenants(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "tenants")
}

func (r *SQLiteRepository) members(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "tenant_members")
}

func (r *SQLiteRepository) invitations(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "tenant_invitations")
}

// Row structs for zorm scanning

type tenantRow struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Settings    *string `json:"settings"`
	Limits      *string `json:"limits"`
	OwnerID     string  `json:"owner_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	DeletedAt   *string `json:"deleted_at"`
}

type memberRow struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	UserID    string  `json:"user_id"`
	Role      string  `json:"role"`
	JoinedAt  string  `json:"joined_at"`
	InvitedBy *string `json:"invited_by"`
}

type invitationRow struct {
	ID         string  `json:"id"`
	TenantID   string  `json:"tenant_id"`
	Email      string  `json:"email"`
	Role       string  `json:"role"`
	Token      string  `json:"token"`
	InvitedBy  string  `json:"invited_by"`
	ExpiresAt  string  `json:"expires_at"`
	AcceptedAt *string `json:"accepted_at"`
	CreatedAt  string  `json:"created_at"`
}

// Converter functions

func parseTimeStr(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

func parseTimePtrStr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t := parseTimeStr(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

func rowToTenant(row tenantRow) *Tenant {
	t := &Tenant{
		Name:        row.Name,
		Slug:        row.Slug,
		Description: row.Description,
		Status:      Status(row.Status),
		Settings:    row.Settings,
		Limits:      row.Limits,
		CreatedAt:   parseTimeStr(row.CreatedAt),
		UpdatedAt:   parseTimeStr(row.UpdatedAt),
		DeletedAt:   parseTimePtrStr(row.DeletedAt),
	}
	t.ID, _ = uuid.Parse(row.ID)
	t.OwnerID, _ = uuid.Parse(row.OwnerID)
	return t
}

func rowToMember(row memberRow) *TenantMember {
	m := &TenantMember{
		Role:     MemberRole(row.Role),
		JoinedAt: parseTimeStr(row.JoinedAt),
	}
	m.ID, _ = uuid.Parse(row.ID)
	m.TenantID, _ = uuid.Parse(row.TenantID)
	m.UserID, _ = uuid.Parse(row.UserID)
	if row.InvitedBy != nil {
		invitedBy, _ := uuid.Parse(*row.InvitedBy)
		m.InvitedBy = &invitedBy
	}
	return m
}

func rowToInvitation(row invitationRow) *TenantInvitation {
	inv := &TenantInvitation{
		Email:      row.Email,
		Role:       MemberRole(row.Role),
		Token:      row.Token,
		ExpiresAt:  parseTimeStr(row.ExpiresAt),
		AcceptedAt: parseTimePtrStr(row.AcceptedAt),
		CreatedAt:  parseTimeStr(row.CreatedAt),
	}
	inv.ID, _ = uuid.Parse(row.ID)
	inv.TenantID, _ = uuid.Parse(row.TenantID)
	inv.InvitedBy, _ = uuid.Parse(row.InvitedBy)
	return inv
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
	_, err := r.tenants(ctx).Insert(map[string]interface{}{
		"id":          tenant.ID.String(),
		"name":        tenant.Name,
		"slug":        tenant.Slug,
		"description": tenant.Description,
		"status":      tenant.Status,
		"settings":    tenant.Settings,
		"limits":      tenant.Limits,
		"owner_id":    tenant.OwnerID.String(),
		"created_at":  tenant.CreatedAt,
		"updated_at":  tenant.UpdatedAt,
	})
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
	var rows []tenantRow
	_, err := r.tenants(ctx).Select(&rows,
		z.Where(z.Eq("id", id.String()), z.IsNull("deleted_at")),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrTenantNotFound
	}
	return rowToTenant(rows[0]), nil
}

// GetBySlug retrieves a tenant by slug.
func (r *SQLiteRepository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var rows []tenantRow
	_, err := r.tenants(ctx).Select(&rows,
		z.Where(z.Eq("slug", slug), z.IsNull("deleted_at")),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrTenantNotFound
	}
	return rowToTenant(rows[0]), nil
}

// Update updates a tenant.
func (r *SQLiteRepository) Update(ctx context.Context, tenant *Tenant) error {
	n, err := r.tenants(ctx).Update(
		map[string]interface{}{
			"name":        tenant.Name,
			"description": tenant.Description,
			"status":      tenant.Status,
			"settings":    tenant.Settings,
			"limits":      tenant.Limits,
			"updated_at":  timeutil.NowTime().UTC(),
		},
		z.Where(z.Eq("id", tenant.ID.String()), z.IsNull("deleted_at")),
	)
	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}
	if n == 0 {
		return ErrTenantNotFound
	}
	return nil
}

// Delete soft-deletes a tenant.
func (r *SQLiteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.tenants(ctx).Update(
		map[string]interface{}{
			"deleted_at": timeutil.NowTime().UTC(),
			"status":     StatusDeleted,
		},
		z.Where(z.Eq("id", id.String()), z.IsNull("deleted_at")),
	)
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}
	if n == 0 {
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
		t := &Tenant{}
		var idStr, ownerIDStr string
		if err := rows.Scan(
			&idStr,
			&t.Name,
			&t.Slug,
			&t.Description,
			&t.Status,
			&t.Settings,
			&t.Limits,
			&ownerIDStr,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}
		t.ID, _ = uuid.Parse(idStr)
		t.OwnerID, _ = uuid.Parse(ownerIDStr)
		tenants = append(tenants, t)
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
		t := &Tenant{}
		var idStr, ownerIDStr string
		if err := rows.Scan(
			&idStr,
			&t.Name,
			&t.Slug,
			&t.Description,
			&t.Status,
			&t.Settings,
			&t.Limits,
			&ownerIDStr,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}
		t.ID, _ = uuid.Parse(idStr)
		t.OwnerID, _ = uuid.Parse(ownerIDStr)
		tenants = append(tenants, t)
	}

	return tenants, nil
}

// AddMember adds a member to a tenant.
func (r *SQLiteRepository) AddMember(ctx context.Context, member *TenantMember) error {
	var invitedBy *string
	if member.InvitedBy != nil {
		s := member.InvitedBy.String()
		invitedBy = &s
	}

	_, err := r.members(ctx).Insert(map[string]interface{}{
		"id":         member.ID.String(),
		"tenant_id":  member.TenantID.String(),
		"user_id":    member.UserID.String(),
		"role":       member.Role,
		"joined_at":  member.JoinedAt,
		"invited_by": invitedBy,
	})
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
	var rows []memberRow
	_, err := r.members(ctx).Select(&rows,
		z.Where(z.Eq("tenant_id", tenantID.String()), z.Eq("user_id", userID.String())),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get member: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrMemberNotFound
	}
	return rowToMember(rows[0]), nil
}

// UpdateMember updates a member's role.
func (r *SQLiteRepository) UpdateMember(ctx context.Context, member *TenantMember) error {
	n, err := r.members(ctx).Update(
		map[string]interface{}{"role": member.Role},
		z.Where(z.Eq("tenant_id", member.TenantID.String()), z.Eq("user_id", member.UserID.String())),
	)
	if err != nil {
		return fmt.Errorf("failed to update member: %w", err)
	}
	if n == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// RemoveMember removes a member from a tenant.
func (r *SQLiteRepository) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	n, err := r.members(ctx).Delete(
		z.Where(z.Eq("tenant_id", tenantID.String()), z.Eq("user_id", userID.String())),
	)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	if n == 0 {
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
	_, err := r.invitations(ctx).Insert(map[string]interface{}{
		"id":         invitation.ID.String(),
		"tenant_id":  invitation.TenantID.String(),
		"email":      invitation.Email,
		"role":       invitation.Role,
		"token":      invitation.Token,
		"invited_by": invitation.InvitedBy.String(),
		"expires_at": invitation.ExpiresAt,
		"created_at": invitation.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}
	return nil
}

// GetInvitationByID retrieves an invitation by ID.
func (r *SQLiteRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*TenantInvitation, error) {
	var rows []invitationRow
	_, err := r.invitations(ctx).Select(&rows,
		z.Where(z.Eq("id", id.String())),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrInvitationNotFound
	}
	return rowToInvitation(rows[0]), nil
}

// GetInvitationByToken retrieves an invitation by token.
func (r *SQLiteRepository) GetInvitationByToken(ctx context.Context, token string) (*TenantInvitation, error) {
	var rows []invitationRow
	_, err := r.invitations(ctx).Select(&rows,
		z.Where(z.Eq("token", token)),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrInvitationNotFound
	}
	return rowToInvitation(rows[0]), nil
}

// GetPendingInvitationByEmail retrieves a pending invitation by email for a tenant.
func (r *SQLiteRepository) GetPendingInvitationByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*TenantInvitation, error) {
	var rows []invitationRow
	_, err := r.invitations(ctx).Select(&rows,
		z.Where(
			z.Eq("tenant_id", tenantID.String()),
			z.Eq("email", email),
			z.IsNull("accepted_at"),
			z.Gt("expires_at", timeutil.NowTime().UTC()),
		),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrInvitationNotFound
	}
	return rowToInvitation(rows[0]), nil
}

// AcceptInvitation marks an invitation as accepted.
func (r *SQLiteRepository) AcceptInvitation(ctx context.Context, id uuid.UUID) error {
	n, err := r.invitations(ctx).Update(
		map[string]interface{}{"accepted_at": timeutil.NowTime().UTC()},
		z.Where(z.Eq("id", id.String()), z.IsNull("accepted_at")),
	)
	if err != nil {
		return fmt.Errorf("failed to accept invitation: %w", err)
	}
	if n == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

// DeleteInvitation deletes an invitation.
func (r *SQLiteRepository) DeleteInvitation(ctx context.Context, id uuid.UUID) error {
	n, err := r.invitations(ctx).Delete(
		z.Where(z.Eq("id", id.String())),
	)
	if err != nil {
		return fmt.Errorf("failed to delete invitation: %w", err)
	}
	if n == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

// ListPendingInvitations retrieves pending invitations for a tenant.
func (r *SQLiteRepository) ListPendingInvitations(ctx context.Context, tenantID uuid.UUID) ([]*TenantInvitation, error) {
	var rows []invitationRow
	_, err := r.invitations(ctx).Select(&rows,
		z.Where(
			z.Eq("tenant_id", tenantID.String()),
			z.IsNull("accepted_at"),
			z.Gt("expires_at", timeutil.NowTime().UTC()),
		),
		z.OrderBy("created_at DESC"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list invitations: %w", err)
	}

	invs := make([]*TenantInvitation, len(rows))
	for i, row := range rows {
		invs[i] = rowToInvitation(row)
	}
	return invs, nil
}

// CleanupExpiredInvitations removes expired invitations.
func (r *SQLiteRepository) CleanupExpiredInvitations(ctx context.Context) error {
	_, err := r.invitations(ctx).Delete(
		z.Where(
			z.Lt("expires_at", timeutil.NowTime().UTC()),
			z.IsNull("accepted_at"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to cleanup invitations: %w", err)
	}
	return nil
}
