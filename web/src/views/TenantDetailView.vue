<template>
  <div class="tenant-detail-view">
    <div class="header">
      <button class="back-btn" @click="goBack">← Back</button>
      <div class="header-info">
        <h1>{{ tenant?.name || 'Loading...' }}</h1>
        <span v-if="tenant" class="tenant-slug">{{ tenant.slug }}</span>
      </div>
    </div>

    <div v-if="loading" class="loading">Loading...</div>
    <div v-else-if="!tenant" class="error">Workspace not found</div>
    <template v-else>
      <!-- Tabs -->
      <div class="tabs">
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'members' }"
          @click="activeTab = 'members'"
        >
          Members
        </button>
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'invitations' }"
          @click="activeTab = 'invitations'"
        >
          Invitations
        </button>
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'settings' }"
          @click="activeTab = 'settings'"
        >
          Settings
        </button>
        <button
          v-if="isOwner"
          class="tab-btn"
          :class="{ active: activeTab === 'limits' }"
          @click="activeTab = 'limits'"
        >
          Limits
        </button>
      </div>

      <!-- Members Tab -->
      <div v-if="activeTab === 'members'" class="tab-content">
        <div class="section-header">
          <h2>Members</h2>
          <button v-if="isAdmin" class="btn btn-primary" @click="showInviteModal = true">
            + Invite Member
          </button>
        </div>

        <div class="members-list">
          <div v-for="member in members" :key="member.id" class="member-card">
            <div class="member-avatar">{{ getInitial(member.username) }}</div>
            <div class="member-info">
              <span class="member-name">{{ member.username }}</span>
              <span v-if="member.email" class="member-email">{{ member.email }}</span>
            </div>
            <span class="member-role" :style="{ color: getRoleColor(member.role) }">
              {{ getRoleLabel(member.role) }}
            </span>
            <div v-if="isAdmin && member.role !== 'owner'" class="member-actions">
              <select
                :value="member.role"
                @change="updateMemberRole(member.user_id, ($event.target as HTMLSelectElement).value as MemberRole)"
              >
                <option value="admin">Admin</option>
                <option value="member">Member</option>
              </select>
              <button class="btn btn-sm btn-danger" @click="confirmRemoveMember(member)">
                Remove
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Invitations Tab -->
      <div v-if="activeTab === 'invitations'" class="tab-content">
        <div class="section-header">
          <h2>Pending Invitations</h2>
          <button v-if="isAdmin" class="btn btn-primary" @click="showInviteModal = true">
            + Invite Member
          </button>
        </div>

        <div v-if="invitations.length === 0" class="empty-state">
          <p>No pending invitations</p>
        </div>
        <div v-else class="invitations-list">
          <div v-for="invitation in invitations" :key="invitation.id" class="invitation-card">
            <div class="invitation-info">
              <span class="invitation-email">{{ invitation.email }}</span>
              <span class="invitation-role" :style="{ color: getRoleColor(invitation.role) }">
                {{ getRoleLabel(invitation.role) }}
              </span>
            </div>
            <div class="invitation-meta">
              <span>Expires {{ formatDate(invitation.expires_at) }}</span>
            </div>
            <button
              v-if="isAdmin"
              class="btn btn-sm btn-danger"
              @click="cancelInvitation(invitation.id)"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>

      <!-- Settings Tab -->
      <div v-if="activeTab === 'settings'" class="tab-content">
        <div class="section-header">
          <h2>Settings</h2>
        </div>

        <div class="settings-form">
          <div class="form-group">
            <label for="language">Default Language</label>
            <select id="language" v-model="settingsForm.default_language" :disabled="!isAdmin">
              <option value="en">English</option>
              <option value="zh">Chinese</option>
              <option value="ja">Japanese</option>
              <option value="ko">Korean</option>
            </select>
          </div>

          <div class="form-group">
            <label for="timezone">Timezone</label>
            <select id="timezone" v-model="settingsForm.timezone" :disabled="!isAdmin">
              <option value="UTC">UTC</option>
              <option value="America/New_York">Eastern Time</option>
              <option value="America/Los_Angeles">Pacific Time</option>
              <option value="Europe/London">London</option>
              <option value="Asia/Shanghai">Shanghai</option>
              <option value="Asia/Tokyo">Tokyo</option>
            </select>
          </div>

          <div v-if="isAdmin" class="form-actions">
            <button class="btn btn-primary" :disabled="savingSettings" @click="saveSettings">
              {{ savingSettings ? 'Saving...' : 'Save Settings' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Limits Tab -->
      <div v-if="activeTab === 'limits' && isOwner" class="tab-content">
        <div class="section-header">
          <h2>Resource Limits</h2>
        </div>

        <div class="limits-grid">
          <div class="limit-card">
            <div class="limit-label">Max Users</div>
            <div class="limit-value">{{ limits?.max_users || 0 }}</div>
          </div>
          <div class="limit-card">
            <div class="limit-label">Max Storage</div>
            <div class="limit-value">{{ formatStorage(limits?.max_storage || 0) }}</div>
          </div>
          <div class="limit-card">
            <div class="limit-label">Max API Requests/Day</div>
            <div class="limit-value">{{ limits?.max_api_requests || 0 }}</div>
          </div>
          <div class="limit-card">
            <div class="limit-label">Max Workflows</div>
            <div class="limit-value">{{ limits?.max_workflows || 0 }}</div>
          </div>
          <div class="limit-card">
            <div class="limit-label">Max Channels</div>
            <div class="limit-value">{{ limits?.max_channels || 0 }}</div>
          </div>
        </div>
      </div>
    </template>

    <!-- Invite Modal -->
    <div v-if="showInviteModal" class="modal-overlay" @click="showInviteModal = false">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h2>Invite Member</h2>
          <button class="close-btn" @click="showInviteModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label for="invite-email">Email Address</label>
            <input
              id="invite-email"
              v-model="inviteForm.email"
              type="email"
              placeholder="user@example.com"
            />
          </div>
          <div class="form-group">
            <label for="invite-role">Role</label>
            <select id="invite-role" v-model="inviteForm.role">
              <option value="member">Member</option>
              <option value="admin">Admin</option>
            </select>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showInviteModal = false">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="!canInvite || inviting"
            @click="handleInvite"
          >
            {{ inviting ? 'Sending...' : 'Send Invitation' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Remove Member Confirmation -->
    <div v-if="removingMember" class="modal-overlay" @click="removingMember = null">
      <div class="modal modal-sm" @click.stop>
        <div class="modal-header">
          <h2>Remove Member</h2>
          <button class="close-btn" @click="removingMember = null">&times;</button>
        </div>
        <div class="modal-body">
          <p>Are you sure you want to remove <strong>{{ removingMember.username }}</strong> from this workspace?</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="removingMember = null">Cancel</button>
          <button class="btn btn-danger" :disabled="removing" @click="handleRemoveMember">
            {{ removing ? 'Removing...' : 'Remove' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTenantStore } from '@/stores/tenant'
import type { TenantMember, TenantInvitation, MemberRole, TenantSettings, TenantLimits } from '@/api/tenant'
import { getRoleLabel, getRoleColor, formatStorageSize } from '@/api/tenant'
import * as tenantApi from '@/api/tenant'

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()

const activeTab = ref<'members' | 'invitations' | 'settings' | 'limits'>('members')
const showInviteModal = ref(false)
const removingMember = ref<TenantMember | null>(null)
const inviting = ref(false)
const removing = ref(false)
const savingSettings = ref(false)

const members = ref<TenantMember[]>([])
const invitations = ref<TenantInvitation[]>([])
const limits = ref<TenantLimits | null>(null)

const inviteForm = ref({
  email: '',
  role: 'member' as MemberRole,
})

const settingsForm = ref<TenantSettings>({
  default_language: 'en',
  timezone: 'UTC',
  features: {},
})

const tenantId = computed(() => route.params.id as string)
const tenant = computed(() => tenantStore.currentTenant)
const loading = computed(() => tenantStore.loading)
const userId = computed(() => localStorage.getItem('user_id'))

const isOwner = computed(() => {
  return tenant.value?.owner_id === userId.value
})

const isAdmin = computed(() => {
  if (isOwner.value) return true
  const member = members.value.find((m) => m.user_id === userId.value)
  return member?.role === 'admin' || member?.role === 'owner'
})

const canInvite = computed(() => {
  return inviteForm.value.email && inviteForm.value.email.includes('@')
})

function getInitial(name: string): string {
  return name.charAt(0).toUpperCase()
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

function formatStorage(bytes: number): string {
  return formatStorageSize(bytes)
}

function goBack() {
  router.push('/tenants')
}

async function loadData() {
  if (!tenantId.value) return

  await tenantStore.selectTenant(tenantId.value)
  await loadMembers()
  await loadInvitations()
  await loadSettings()
  await loadLimits()
}

async function loadMembers() {
  try {
    const response = await tenantApi.listMembers(tenantId.value)
    members.value = response.members
  } catch (error) {
    console.error('Failed to load members:', error)
  }
}

async function loadInvitations() {
  try {
    const response = await tenantApi.listInvitations(tenantId.value)
    invitations.value = response.invitations
  } catch (error) {
    console.error('Failed to load invitations:', error)
  }
}

async function loadSettings() {
  try {
    const settings = await tenantApi.getTenantSettings(tenantId.value)
    settingsForm.value = settings
  } catch (error) {
    console.error('Failed to load settings:', error)
  }
}

async function loadLimits() {
  try {
    limits.value = await tenantApi.getTenantLimits(tenantId.value)
  } catch (error) {
    console.error('Failed to load limits:', error)
  }
}

async function handleInvite() {
  inviting.value = true
  try {
    await tenantApi.inviteMember(tenantId.value, {
      email: inviteForm.value.email,
      role: inviteForm.value.role,
    })
    showInviteModal.value = false
    inviteForm.value = { email: '', role: 'member' }
    await loadInvitations()
  } catch (error) {
    console.error('Failed to invite member:', error)
  } finally {
    inviting.value = false
  }
}

async function updateMemberRole(memberId: string, role: MemberRole) {
  try {
    await tenantApi.updateMember(tenantId.value, memberId, { role })
    await loadMembers()
  } catch (error) {
    console.error('Failed to update member role:', error)
  }
}

function confirmRemoveMember(member: TenantMember) {
  removingMember.value = member
}

async function handleRemoveMember() {
  if (!removingMember.value) return

  removing.value = true
  try {
    await tenantApi.removeMember(tenantId.value, removingMember.value.user_id)
    removingMember.value = null
    await loadMembers()
  } catch (error) {
    console.error('Failed to remove member:', error)
  } finally {
    removing.value = false
  }
}

async function cancelInvitation(invitationId: string) {
  try {
    await tenantApi.cancelInvitation(tenantId.value, invitationId)
    await loadInvitations()
  } catch (error) {
    console.error('Failed to cancel invitation:', error)
  }
}

async function saveSettings() {
  savingSettings.value = true
  try {
    await tenantApi.updateTenantSettings(tenantId.value, settingsForm.value)
  } catch (error) {
    console.error('Failed to save settings:', error)
  } finally {
    savingSettings.value = false
  }
}

watch(tenantId, () => {
  loadData()
})

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.tenant-detail-view {
  padding: 1.5rem;
  max-width: 1000px;
  margin: 0 auto;
}

.header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.back-btn {
  padding: 0.5rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  cursor: pointer;
  font-size: 0.875rem;
}

.header-info h1 {
  margin: 0;
  font-size: 1.5rem;
}

.tenant-slug {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.loading,
.error {
  text-align: center;
  padding: 3rem;
  color: var(--color-text-muted);
}

.tabs {
  display: flex;
  gap: 0.25rem;
  border-bottom: 1px solid var(--color-border);
  margin-bottom: 1.5rem;
}

.tab-btn {
  padding: 0.75rem 1.25rem;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  transition: all 0.2s;
}

.tab-btn.active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.section-header h2 {
  margin: 0;
  font-size: 1.25rem;
}

.members-list,
.invitations-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.member-card,
.invitation-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: var(--color-background-soft);
  border-radius: 0.5rem;
}

.member-avatar {
  width: 2.5rem;
  height: 2.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-primary);
  color: white;
  border-radius: 50%;
  font-weight: 600;
}

.member-info,
.invitation-info {
  flex: 1;
}

.member-name,
.invitation-email {
  display: block;
  font-weight: 500;
}

.member-email {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.member-role,
.invitation-role {
  font-size: 0.75rem;
  font-weight: 500;
}

.member-actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.member-actions select {
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  font-size: 0.75rem;
}

.invitation-meta {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.empty-state {
  text-align: center;
  padding: 2rem;
  color: var(--color-text-muted);
}

.settings-form {
  max-width: 400px;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.375rem;
  font-weight: 500;
  font-size: 0.875rem;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 0.625rem 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  font-size: 0.875rem;
  background: var(--color-background);
}

.form-group select:disabled {
  background: var(--color-background-soft);
  cursor: not-allowed;
}

.form-actions {
  margin-top: 1.5rem;
}

.limits-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 1rem;
}

.limit-card {
  background: var(--color-background-soft);
  border-radius: 0.5rem;
  padding: 1.25rem;
  text-align: center;
}

.limit-label {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  margin-bottom: 0.5rem;
}

.limit-value {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--color-primary);
}

/* Modal styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--color-background);
  border-radius: 0.75rem;
  width: 90%;
  max-width: 450px;
  overflow: hidden;
}

.modal-sm {
  max-width: 380px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--color-border);
}

.modal-header h2 {
  margin: 0;
  font-size: 1.125rem;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--color-text-muted);
}

.modal-body {
  padding: 1.5rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  border-top: 1px solid var(--color-border);
}

/* Button styles */
.btn {
  padding: 0.625rem 1.25rem;
  border: none;
  border-radius: 0.5rem;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--color-primary);
  color: white;
}

.btn-secondary {
  background: var(--color-background-soft);
  color: var(--color-text);
  border: 1px solid var(--color-border);
}

.btn-danger {
  background: #ef4444;
  color: white;
}

.btn-sm {
  padding: 0.375rem 0.75rem;
  font-size: 0.875rem;
}
</style>
