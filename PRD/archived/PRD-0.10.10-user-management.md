# v0.10.10 User Management & Page Permissions PRD

## Overview

Version 0.10.10 introduces a comprehensive user management system that allows administrators to create and manage sub-users with granular page-level permissions. This enables multi-user scenarios where different users have access to different features based on their assigned permissions.

**Key Features**:
- Sub-user creation and management by admin
- Page-level permission system
- Default permission: Chat only (minimal access)
- Role-based access control (RBAC) integration
- User list with status management (active/locked/disabled)

## Problem Statement

### Current Issues

1. **Single User System**
   - Only admin account exists after setup
   - No way to share access with family members or team
   - All-or-nothing access model

2. **No Permission Control**
   - Every user has full access to all features
   - Cannot restrict sensitive pages (Settings, Security, etc.)
   - No audit trail for user actions

3. **Security Concerns**
   - Sharing admin credentials is risky
   - No way to revoke access without changing password
   - Cannot track who did what

## Solution Design

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    User Management Flow                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Admin creates sub-user                                     │
│           ↓                                                 │
│  Assign default permissions (chat only)                     │
│           ↓                                                 │
│  (Optional) Customize page permissions                      │
│           ↓                                                 │
│  Sub-user logs in                                           │
│           ↓                                                 │
│  Frontend checks permissions                                │
│           ↓                                                 │
│  Show/hide navigation items based on permissions            │
│           ↓                                                 │
│  Backend validates API access                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Permission Model

#### Page Permissions

| Permission Key | Page | Default (Sub-user) | Admin |
|---------------|------|-------------------|-------|
| `page.chat` | Chat | ✅ Yes | ✅ Yes |
| `page.home` | Home/Dashboard | ❌ No | ✅ Yes |
| `page.channels` | Channels | ❌ No | ✅ Yes |
| `page.settings` | Settings | ❌ No | ✅ Yes |
| `page.security` | Security | ❌ No | ✅ Yes |
| `page.users` | User Management | ❌ No | ✅ Yes |
| `page.profile` | Profile (own) | ✅ Yes | ✅ Yes |
| `page.providers` | Auth Providers | ❌ No | ✅ Yes |
| `page.automation` | Automation | ❌ No | ✅ Yes |
| `page.plugins` | Plugins | ❌ No | ✅ Yes |
| `page.tools` | Tool Store | ❌ No | ✅ Yes |
| `page.skills` | Skill Store | ❌ No | ✅ Yes |

#### Predefined Roles

| Role | Description | Permissions |
|------|-------------|-------------|
| `admin` | Full access | `*` (all permissions) |
| `user` | Standard user | `page.chat`, `page.profile`, `page.home` |
| `guest` | Minimal access | `page.chat` only |

### Feature Specifications

#### F1: User Management Page

**Description**: New page for administrators to manage sub-users.

**Location**: Settings > Users (or dedicated `/users` route)

**Requirements**:
- List all users with pagination
- Display user info: username, email, role, status, created date, last login
- Actions: Create, Edit, Lock/Unlock, Delete
- Search and filter by status/role
- Bulk actions (optional)

**UI Components**:
```
┌─────────────────────────────────────────────────────────────┐
│  User Management                              [+ Add User]  │
├─────────────────────────────────────────────────────────────┤
│  Search: [________________]  Filter: [All ▼] [Active ▼]    │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 👤 admin          admin    Active    2024-01-01     │   │
│  │    admin@local             Last: 2 hours ago   [⋮]  │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │ 👤 john           user     Active    2024-01-15     │   │
│  │    john@email.com          Last: 1 day ago     [⋮]  │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │ 👤 guest1         guest    Locked    2024-01-20     │   │
│  │    -                       Last: Never         [⋮]  │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  Showing 1-3 of 3 users                    [< 1 2 3 ... >] │
└─────────────────────────────────────────────────────────────┘
```

#### F2: Create User Modal

**Description**: Modal dialog for creating new sub-users.

**Fields**:
- Username (required, 3-50 chars, unique)
- Email (optional, unique if provided)
- Password (required, must meet strength requirements)
- Confirm Password
- Role (dropdown: user, guest)
- Page Permissions (checkboxes, pre-filled based on role)

**Validation**:
- Username: alphanumeric + underscore, 3-50 chars
- Email: valid email format if provided
- Password: 8+ chars, uppercase, lowercase, number, special char
- Passwords must match

#### F3: Edit User Modal

**Description**: Modal dialog for editing existing users.

**Fields**:
- Username (read-only)
- Email (editable)
- Role (dropdown)
- Page Permissions (checkboxes)
- Reset Password (optional section)

**Actions**:
- Save Changes
- Cancel

#### F4: User Status Management

**Description**: Lock, unlock, and disable user accounts.

**Status Types**:
- `active`: Normal access
- `locked`: Temporarily blocked (auto or manual)
- `disabled`: Permanently blocked until re-enabled

**Actions**:
- Lock User: Immediately revoke access
- Unlock User: Restore access
- Disable User: Soft-delete (can be re-enabled)

#### F5: Permission-Based Navigation

**Description**: Frontend navigation adapts based on user permissions.

**Requirements**:
- Sidebar shows only permitted pages
- Direct URL access blocked for unpermitted pages
- API calls return 403 for unpermitted actions
- Graceful redirect to permitted page if accessing blocked route

**Implementation**:
```typescript
// Router guard
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  const requiredPermission = to.meta.permission

  if (requiredPermission && !authStore.hasPermission(requiredPermission)) {
    next('/chat') // Redirect to default permitted page
  } else {
    next()
  }
})
```

#### F6: Permission API

**Description**: Backend API for permission checking.

**Endpoints**:
- `GET /api/v1/users/me/permissions` - Get current user's permissions
- `GET /api/v1/users/:id/permissions` - Get specific user's permissions (admin only)
- `PUT /api/v1/users/:id/permissions` - Update user's permissions (admin only)

**Response Format**:
```json
{
  "permissions": [
    "page.chat",
    "page.profile",
    "page.home"
  ],
  "role": "user"
}
```

## API Endpoints

### User Management

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/api/v1/users` | List all users (paginated) | Admin |
| POST | `/api/v1/users` | Create new user | Admin |
| GET | `/api/v1/users/:id` | Get user details | Admin |
| PUT | `/api/v1/users/:id` | Update user | Admin |
| DELETE | `/api/v1/users/:id` | Delete user (soft) | Admin |
| POST | `/api/v1/users/:id/lock` | Lock user account | Admin |
| POST | `/api/v1/users/:id/unlock` | Unlock user account | Admin |
| POST | `/api/v1/users/:id/reset-password` | Reset user password | Admin |

### Permission Management

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/api/v1/users/me/permissions` | Get own permissions | Any |
| GET | `/api/v1/users/:id/permissions` | Get user permissions | Admin |
| PUT | `/api/v1/users/:id/permissions` | Update permissions | Admin |
| GET | `/api/v1/permissions/available` | List all available permissions | Admin |

### Request/Response Examples

#### Create User
```http
POST /api/v1/users
Content-Type: application/json

{
  "username": "john",
  "email": "john@example.com",
  "password": "SecurePass123!",
  "role": "user",
  "permissions": ["page.chat", "page.profile", "page.home"]
}
```

#### Response
```json
{
  "success": true,
  "user": {
    "id": "uuid-here",
    "username": "john",
    "email": "john@example.com",
    "role": "user",
    "status": "active",
    "permissions": ["page.chat", "page.profile", "page.home"],
    "created_at": "2024-01-31T10:00:00Z"
  }
}
```

#### List Users
```http
GET /api/v1/users?page=1&limit=10&status=active&role=user
```

#### Response
```json
{
  "success": true,
  "users": [
    {
      "id": "uuid-1",
      "username": "john",
      "email": "john@example.com",
      "role": "user",
      "status": "active",
      "last_login_at": "2024-01-30T15:00:00Z",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 1,
    "total_pages": 1
  }
}
```

## Database Schema

### Existing Tables (from user package)

```sql
-- users table already exists with role field
-- Adding permissions via user_permissions table

CREATE TABLE user_permissions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    permission TEXT NOT NULL,
    granted_by TEXT,
    granted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, permission),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_user_permissions_user_id ON user_permissions(user_id);
```

### Permission Resolution

1. Check if user is admin → grant all permissions
2. Check user's role → get role's default permissions
3. Check user_permissions table → get custom permissions
4. Merge and deduplicate

## Frontend Implementation

### Auth Store Extensions

```typescript
// stores/auth.ts
interface AuthState {
  user: User | null
  token: string | null
  permissions: string[]
}

const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    user: null,
    token: null,
    permissions: []
  }),

  getters: {
    hasPermission: (state) => (permission: string) => {
      if (state.user?.role === 'admin') return true
      return state.permissions.includes(permission)
    },

    isAdmin: (state) => state.user?.role === 'admin'
  },

  actions: {
    async fetchPermissions() {
      const response = await api.get('/users/me/permissions')
      this.permissions = response.data.permissions
    }
  }
})
```

### Route Configuration

```typescript
// router/index.ts
const routes = [
  {
    path: '/chat',
    component: ChatView,
    meta: { permission: 'page.chat' }
  },
  {
    path: '/home',
    component: HomeView,
    meta: { permission: 'page.home' }
  },
  {
    path: '/users',
    component: UsersView,
    meta: { permission: 'page.users' }
  },
  // ... other routes
]
```

### Sidebar Component

```vue
<!-- AppSidebar.vue -->
<template>
  <nav>
    <SidebarItem
      v-if="hasPermission('page.chat')"
      to="/chat"
      icon="chat"
      :label="t('nav.chat')"
    />
    <SidebarItem
      v-if="hasPermission('page.home')"
      to="/home"
      icon="home"
      :label="t('nav.home')"
    />
    <!-- Admin-only items -->
    <SidebarItem
      v-if="isAdmin"
      to="/users"
      icon="users"
      :label="t('nav.users')"
    />
  </nav>
</template>
```

## Security Considerations

### Password Security
- Argon2id hashing (already implemented)
- Password strength validation (8+ chars, mixed case, numbers, special)
- Password history to prevent reuse (already implemented)

### Session Security
- JWT tokens with expiration
- Refresh token rotation
- Session tracking per device
- Automatic logout on password change

### Permission Security
- Backend validation for all API calls
- Frontend checks are UX only, not security
- Admin cannot delete themselves
- Cannot escalate own permissions

### Audit Trail
- Log all user management actions
- Track permission changes
- Record login attempts (success/failure)

## User Experience

### Admin Flow

1. **Navigate to User Management**
   - Click "Users" in sidebar (admin only)
   - See list of all users

2. **Create New User**
   - Click "Add User" button
   - Fill in username, email, password
   - Select role (user/guest)
   - Customize permissions if needed
   - Click "Create"

3. **Edit User Permissions**
   - Click user row or edit button
   - Modify permissions checkboxes
   - Save changes

4. **Lock/Unlock User**
   - Click menu (⋮) on user row
   - Select "Lock" or "Unlock"
   - Confirm action

### Sub-user Flow

1. **Login**
   - Enter username and password
   - Redirected to Chat (default page)

2. **Navigation**
   - See only permitted pages in sidebar
   - Attempting blocked routes redirects to Chat

3. **Profile**
   - Can view and edit own profile
   - Can change own password

## i18n Keys

```typescript
// en-US.ts
users: {
  title: 'User Management',
  addUser: 'Add User',
  editUser: 'Edit User',
  deleteUser: 'Delete User',
  username: 'Username',
  email: 'Email',
  role: 'Role',
  status: 'Status',
  permissions: 'Permissions',
  lastLogin: 'Last Login',
  createdAt: 'Created',
  actions: 'Actions',
  lock: 'Lock',
  unlock: 'Unlock',
  disable: 'Disable',
  enable: 'Enable',
  resetPassword: 'Reset Password',
  confirmDelete: 'Are you sure you want to delete this user?',
  deleteWarning: 'This action cannot be undone.',
  noUsers: 'No users found',
  searchPlaceholder: 'Search users...',
  filterByStatus: 'Filter by status',
  filterByRole: 'Filter by role',
  allStatuses: 'All statuses',
  allRoles: 'All roles',
  statusActive: 'Active',
  statusLocked: 'Locked',
  statusDisabled: 'Disabled',
  roleAdmin: 'Administrator',
  roleUser: 'User',
  roleGuest: 'Guest',
  permissionPages: 'Page Access',
  permissionChat: 'Chat',
  permissionHome: 'Dashboard',
  permissionChannels: 'Channels',
  permissionSettings: 'Settings',
  permissionSecurity: 'Security',
  permissionUsers: 'User Management',
  permissionProfile: 'Profile',
  permissionProviders: 'Auth Providers',
  permissionAutomation: 'Automation',
  permissionPlugins: 'Plugins',
  permissionTools: 'Tool Store',
  permissionSkills: 'Skill Store',
  createSuccess: 'User created successfully',
  updateSuccess: 'User updated successfully',
  deleteSuccess: 'User deleted successfully',
  lockSuccess: 'User locked successfully',
  unlockSuccess: 'User unlocked successfully',
  passwordResetSuccess: 'Password reset successfully',
  error: {
    usernameExists: 'Username already exists',
    emailExists: 'Email already exists',
    invalidPassword: 'Password does not meet requirements',
    cannotDeleteSelf: 'Cannot delete your own account',
    cannotLockSelf: 'Cannot lock your own account',
  }
}
```

## Testing Strategy

### Unit Tests
- [ ] User service: CRUD operations
- [ ] Permission service: permission checking
- [ ] API handlers: request/response validation
- [ ] Frontend: permission guards

### Integration Tests
- [ ] Create user → login → access permitted pages
- [ ] Create user → access blocked pages → redirect
- [ ] Lock user → login fails
- [ ] Admin creates user with custom permissions

### Manual Testing
- [ ] Full admin flow (create, edit, lock, delete)
- [ ] Sub-user login and navigation
- [ ] Permission changes take effect immediately
- [ ] Password reset flow

## Success Metrics

- [ ] Admin can create sub-users
- [ ] Sub-users can only access permitted pages
- [ ] Permission changes are immediate
- [ ] No security bypasses via direct URL
- [ ] Audit trail captures all actions

## Timeline

- **Phase 1**: Backend API (user CRUD, permissions)
- **Phase 2**: Frontend User Management page
- **Phase 3**: Permission-based navigation
- **Phase 4**: Testing and polish

## Dependencies

### Backend
- Existing user package (`server/internal/user/`)
- Existing RBAC package (`server/internal/rbac/`)
- Existing auth middleware

### Frontend
- Vue Router (route guards)
- Pinia (auth store)
- Existing UI components

## Future Improvements

### v0.10.11 (Planned)
- User groups for bulk permission management
- Permission templates
- Activity log viewer
- Export user list

### v0.10.12 (Future)
- LDAP/AD integration
- SSO support (OAuth2, SAML)
- Two-factor authentication for sub-users
- API key management per user
