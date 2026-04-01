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
	db     *sql.DB
	readDB *sql.DB
}

// NewSQLiteRepository creates a new SQLiteRepository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return NewSQLiteRepositoryWithReadDB(db, db)
}

// NewSQLiteRepositoryWithReadDB creates a new SQLiteRepository with separate
// write and read database handles.
func NewSQLiteRepositoryWithReadDB(writeDB, readDB *sql.DB) *SQLiteRepository {
	if readDB == nil {
		readDB = writeDB
	}
	return &SQLiteRepository{db: writeDB, readDB: readDB}
}

// Table helpers

func (r *SQLiteRepository) tenants(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "tenants")
}

func (r *SQLiteRepository) tenantsRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.reader(), "tenants")
}

func (r *SQLiteRepository) members(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "tenant_members")
}

func (r *SQLiteRepository) membersRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.reader(), "tenant_members")
}

func (r *SQLiteRepository) invitations(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "tenant_invitations")
}

func (r *SQLiteRepository) invitationsRead(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.reader(), "tenant_invitations")
}

func (r *SQLiteRepository) reader() *sql.DB {
	if r != nil && r.readDB != nil {
		return r.readDB
	}
	if r == nil {
		return nil
	}
	return r.db
}

// Row structs for zorm scanning

type tenantRow struct {
	ID          string  `json:"id" zorm:"id"`
	Name        string  `json:"name" zorm:"name"`
	Slug        string  `json:"slug" zorm:"slug"`
	Description *string `json:"description" zorm:"description"`
	Status      string  `json:"status" zorm:"status"`
	Settings    *string `json:"settings" zorm:"settings"`
	Limits      *string `json:"limits" zorm:"limits"`
	OwnerID     string  `json:"owner_id" zorm:"owner_id"`
	CreatedAt   string  `json:"created_at" zorm:"created_at"`
	UpdatedAt   string  `json:"updated_at" zorm:"updated_at"`
	DeletedAt   *string `json:"deleted_at" zorm:"deleted_at"`
}

type memberRow struct {
	ID        string  `json:"id" zorm:"id"`
	TenantID  string  `json:"tenant_id" zorm:"tenant_id"`
	UserID    string  `json:"user_id" zorm:"user_id"`
	Role      string  `json:"role" zorm:"role"`
	JoinedAt  string  `json:"joined_at" zorm:"joined_at"`
	InvitedBy *string `json:"invited_by" zorm:"invited_by"`
}

type invitationRow struct {
	ID         string  `json:"id" zorm:"id"`
	TenantID   string  `json:"tenant_id" zorm:"tenant_id"`
	Email      string  `json:"email" zorm:"email"`
	Role       string  `json:"role" zorm:"role"`
	Token      string  `json:"token" zorm:"token"`
	InvitedBy  string  `json:"invited_by" zorm:"invited_by"`
	ExpiresAt  string  `json:"expires_at" zorm:"expires_at"`
	AcceptedAt *string `json:"accepted_at" zorm:"accepted_at"`
	CreatedAt  string  `json:"created_at" zorm:"created_at"`
}

type memberWithUserRow struct {
	ID        string  `json:"id" zorm:"id"`
	TenantID  string  `json:"tenant_id" zorm:"tenant_id"`
	UserID    string  `json:"user_id" zorm:"user_id"`
	Role      string  `json:"role" zorm:"role"`
	JoinedAt  string  `json:"joined_at" zorm:"joined_at"`
	InvitedBy *string `json:"invited_by" zorm:"invited_by"`
	Username  string  `json:"username" zorm:"username"`
	Email     string  `json:"email" zorm:"email"`
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

func rowToMemberWithUser(row memberWithUserRow) *TenantMemberWithUser {
	member := &TenantMemberWithUser{
		TenantMember: TenantMember{
			Role:     MemberRole(row.Role),
			JoinedAt: parseTimeStr(row.JoinedAt),
		},
		Username: row.Username,
	}
	member.ID, _ = uuid.Parse(row.ID)
	member.TenantID, _ = uuid.Parse(row.TenantID)
	member.UserID, _ = uuid.Parse(row.UserID)
	if row.Email != "" {
		email := row.Email
		member.Email = &email
	}
	if row.InvitedBy != nil {
		invitedBy, _ := uuid.Parse(*row.InvitedBy)
		member.InvitedBy = &invitedBy
	}
	return member
}

func stringFromMapValue(m z.V, key string) string {
	value, ok := valueFromMapKey(m, key)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339)
	default:
		return fmt.Sprint(typed)
	}
}

func valueFromMapKey(m z.V, key string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	if value, ok := m[key]; ok {
		return value, true
	}

	for rawKey, value := range m {
		if normalizeMapKey(rawKey) == key {
			return value, true
		}
	}

	return nil, false
}

func normalizeMapKey(key string) string {
	key = strings.TrimSpace(strings.Trim(key, "`"))
	if key == "" {
		return ""
	}

	fields := strings.Fields(key)
	if len(fields) >= 3 && strings.EqualFold(fields[len(fields)-2], "as") {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if len(fields) >= 2 {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if dot := strings.LastIndex(key, "."); dot >= 0 {
		return strings.Trim(key[dot+1:], "`")
	}
	return key
}

func tenantFromMapRow(row z.V) *Tenant {
	return rowToTenant(tenantRow{
		ID:          stringFromMapValue(row, "id"),
		Name:        stringFromMapValue(row, "name"),
		Slug:        stringFromMapValue(row, "slug"),
		Description: ptrOrNilString(stringFromMapValue(row, "description")),
		Status:      stringFromMapValue(row, "status"),
		Settings:    ptrOrNilString(stringFromMapValue(row, "settings")),
		Limits:      ptrOrNilString(stringFromMapValue(row, "limits")),
		OwnerID:     stringFromMapValue(row, "owner_id"),
		CreatedAt:   stringFromMapValue(row, "created_at"),
		UpdatedAt:   stringFromMapValue(row, "updated_at"),
		DeletedAt:   ptrOrNilString(stringFromMapValue(row, "deleted_at")),
	})
}

func memberWithUserFromMapRow(row z.V) *TenantMemberWithUser {
	email := ptrOrNilString(stringFromMapValue(row, "email"))
	return &TenantMemberWithUser{
		TenantMember: TenantMember{
			ID:        parseUUIDString(stringFromMapValue(row, "id")),
			TenantID:  parseUUIDString(stringFromMapValue(row, "tenant_id")),
			UserID:    parseUUIDString(stringFromMapValue(row, "user_id")),
			Role:      MemberRole(stringFromMapValue(row, "role")),
			JoinedAt:  parseTimeStr(stringFromMapValue(row, "joined_at")),
			InvitedBy: parseUUIDPtrString(ptrOrNilString(stringFromMapValue(row, "invited_by"))),
		},
		Username: stringFromMapValue(row, "username"),
		Email:    email,
	}
}

func ptrOrNilString(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func parseUUIDString(value string) uuid.UUID {
	id, _ := uuid.Parse(value)
	return id
}

func parseUUIDPtrString(value *string) *uuid.UUID {
	if value == nil {
		return nil
	}
	id, err := uuid.Parse(*value)
	if err != nil {
		return nil
	}
	return &id
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
	_, err := r.tenantsRead(ctx).Select(&rows,
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
	_, err := r.tenantsRead(ctx).Select(&rows,
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
		z.V{
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
		z.V{
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
	if query == nil {
		query = &ListTenantsQuery{}
	}
	// Set defaults
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	conds := []interface{}{z.IsNull("deleted_at")}
	if query.Search != "" {
		searchPattern := "%" + query.Search + "%"
		conds = append(conds, z.Or(z.Like("name", searchPattern), z.Like("slug", searchPattern)))
	}

	if query.Status != nil {
		conds = append(conds, z.Eq("status", string(*query.Status)))
	}

	var total int64
	_, err := r.tenantsRead(ctx).Select(&total, z.Fields("count(1)"), z.Where(conds...))
	if err != nil {
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

	offset := (query.Page - 1) * query.PageSize
	var rows []tenantRow
	_, err = r.tenantsRead(ctx).Select(&rows,
		z.Where(conds...),
		z.OrderBy(fmt.Sprintf("%s %s", orderBy, orderDir)),
		z.Limit(query.PageSize, offset),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	tenants := make([]*Tenant, 0)
	for _, row := range rows {
		tenants = append(tenants, rowToTenant(row))
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
	var total int64
	_, err := r.tenantsRead(ctx).Select(&total,
		z.Fields("count(1)"),
		z.Where(z.Eq("slug", slug), z.IsNull("deleted_at")),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check slug existence: %w", err)
	}
	return total > 0, nil
}

// GetUserTenants retrieves all tenants a user belongs to.
func (r *SQLiteRepository) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*Tenant, error) {
	var rows []z.V
	_, err := z.TableContext(ctx, r.reader(), "tenants t").Select(&rows,
		z.Fields(
			"t.id id",
			"t.name name",
			"t.slug slug",
			"t.description description",
			"t.status status",
			"t.settings settings",
			"t.limits limits",
			"t.owner_id owner_id",
			"t.created_at created_at",
			"t.updated_at updated_at",
			"t.deleted_at deleted_at",
		),
		z.InnerJoin("tenant_members tm", "tm.tenant_id = t.id"),
		z.Where(z.Eq("tm.user_id", userID.String()), z.IsNull("t.deleted_at")),
		z.OrderBy("t.name ASC"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tenants: %w", err)
	}

	tenants := make([]*Tenant, 0)
	for _, row := range rows {
		tenants = append(tenants, tenantFromMapRow(row))
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
	_, err := r.membersRead(ctx).Select(&rows,
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
		z.V{"role": member.Role},
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
	if query == nil {
		query = &ListMembersQuery{}
	}
	// Set defaults
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	conds := []interface{}{
		z.Eq("tm.tenant_id", tenantID.String()),
		z.IsNull("u.deleted_at"),
	}
	if query.Role != nil {
		conds = append(conds, z.Eq("tm.role", string(*query.Role)))
	}

	var total int64
	joined := z.TableContext(ctx, r.reader(), "tenant_members tm")
	_, err := joined.Select(&total,
		z.Fields("count(1)"),
		z.InnerJoin("users u", "u.id = tm.user_id"),
		z.Where(conds...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to count members: %w", err)
	}

	offset := (query.Page - 1) * query.PageSize
	var rows []z.V
	_, err = joined.Select(&rows,
		z.Fields(
			"tm.id id",
			"tm.tenant_id tenant_id",
			"tm.user_id user_id",
			"tm.role role",
			"tm.joined_at joined_at",
			"tm.invited_by invited_by",
			"u.username username",
			"COALESCE(u.email, '') email",
		),
		z.InnerJoin("users u", "u.id = tm.user_id"),
		z.Where(conds...),
		z.OrderBy("tm.joined_at DESC"),
		z.Limit(query.PageSize, offset),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}

	members := make([]*TenantMemberWithUser, 0)
	for _, row := range rows {
		members = append(members, memberWithUserFromMapRow(row))
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
	var count int64
	_, err := r.membersRead(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("tenant_id", tenantID.String())),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to count members: %w", err)
	}
	return count, nil
}

// IsMember checks if a user is a member of a tenant.
func (r *SQLiteRepository) IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var count int64
	_, err := r.membersRead(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("tenant_id", tenantID.String()), z.Eq("user_id", userID.String())),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}
	return count > 0, nil
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
	_, err := r.invitationsRead(ctx).Select(&rows,
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
	_, err := r.invitationsRead(ctx).Select(&rows,
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
	_, err := r.invitationsRead(ctx).Select(&rows,
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
		z.V{"accepted_at": timeutil.NowTime().UTC()},
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
	_, err := r.invitationsRead(ctx).Select(&rows,
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
