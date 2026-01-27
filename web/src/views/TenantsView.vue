<template>
  <div class="tenants-view">
    <div class="header">
      <h1>{{ t('tenants.title') }}</h1>
      <button class="btn btn-primary" @click="showCreateModal = true">
        + {{ t('tenants.newWorkspace') }}
      </button>
    </div>

    <!-- Tenants Grid -->
    <div v-if="loading" class="loading">{{ t('tenants.loadingWorkspaces') }}</div>
    <div v-else-if="tenants.length === 0" class="empty-state">
      <p>{{ t('tenants.noWorkspacesYet') }}</p>
      <button class="btn btn-primary" @click="showCreateModal = true">
        {{ t('tenants.createWorkspace') }}
      </button>
    </div>
    <div v-else class="tenants-grid">
      <div
        v-for="tenant in tenants"
        :key="tenant.id"
        class="tenant-card"
        :class="{ active: tenant.id === currentTenantId }"
        @click="selectTenant(tenant)"
      >
        <div class="tenant-header">
          <span class="tenant-icon">{{ getInitial(tenant.name) }}</span>
          <div class="tenant-info">
            <h3 class="tenant-name">{{ tenant.name }}</h3>
            <span class="tenant-slug">{{ tenant.slug }}</span>
          </div>
          <span class="tenant-status" :style="{ color: getStatusColor(tenant.status) }">
            {{ getStatusLabel(tenant.status) }}
          </span>
        </div>
        <p v-if="tenant.description" class="tenant-description">{{ tenant.description }}</p>
        <div class="tenant-meta">
          <span>{{ t('tenants.created') }} {{ formatDate(tenant.created_at) }}</span>
        </div>
        <div class="tenant-actions" @click.stop>
          <button class="btn btn-sm btn-secondary" @click="editTenant(tenant)">
            {{ t('tenants.edit') }}
          </button>
          <button class="btn btn-sm btn-secondary" @click="manageTenant(tenant)">
            {{ t('tenants.manage') }}
          </button>
          <button
            v-if="tenant.owner_id === userId"
            class="btn btn-sm btn-danger"
            @click="confirmDelete(tenant)"
          >
            {{ t('tenants.delete') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showCreateModal || editingTenant" class="modal-overlay" @click="closeModal">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h2>{{ editingTenant ? t('tenants.editWorkspace') : t('tenants.createWorkspace') }}</h2>
          <button class="close-btn" @click="closeModal">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label for="name">{{ t('tenants.name') }}</label>
            <input
              id="name"
              v-model="formData.name"
              type="text"
              :placeholder="t('tenants.namePlaceholder')"
              @input="!editingTenant && generateSlug()"
            />
          </div>
          <div class="form-group">
            <label for="slug">{{ t('tenants.slug') }}</label>
            <input
              id="slug"
              v-model="formData.slug"
              type="text"
              :placeholder="t('tenants.slugPlaceholder')"
              :disabled="!!editingTenant"
            />
            <p class="hint">{{ t('tenants.slugHint') }}</p>
          </div>
          <div class="form-group">
            <label for="description">{{ t('tenants.description') }}</label>
            <textarea
              id="description"
              v-model="formData.description"
              :placeholder="t('tenants.descriptionPlaceholder')"
            ></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">{{ t('tenants.cancel') }}</button>
          <button
            class="btn btn-primary"
            :disabled="!canSubmit || submitting"
            @click="handleSubmit"
          >
            {{ submitting ? t('tenants.saving') : editingTenant ? t('tenants.save') : t('tenants.create') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="deletingTenant" class="modal-overlay" @click="deletingTenant = null">
      <div class="modal modal-sm" @click.stop>
        <div class="modal-header">
          <h2>{{ t('tenants.deleteWorkspace') }}</h2>
          <button class="close-btn" @click="deletingTenant = null">&times;</button>
        </div>
        <div class="modal-body">
          <p v-html="t('tenants.deleteConfirm', { name: deletingTenant.name })"></p>
          <p class="warning">{{ t('tenants.deleteWarning') }}</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="deletingTenant = null">{{ t('tenants.cancel') }}</button>
          <button
            class="btn btn-danger"
            :disabled="deleting"
            @click="handleDelete"
          >
            {{ deleting ? t('tenants.deleting') : t('tenants.delete') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useTenantStore } from '@/stores/tenant'
import type { Tenant } from '@/api/tenant'
import { getStatusLabel, getStatusColor } from '@/api/tenant'

const { t } = useI18n()

const router = useRouter()
const tenantStore = useTenantStore()

const showCreateModal = ref(false)
const editingTenant = ref<Tenant | null>(null)
const deletingTenant = ref<Tenant | null>(null)
const submitting = ref(false)
const deleting = ref(false)

const formData = ref({
  name: '',
  slug: '',
  description: '',
})

const tenants = computed(() => tenantStore.tenants)
const currentTenantId = computed(() => tenantStore.currentTenantId)
const loading = computed(() => tenantStore.loading)
const userId = computed(() => localStorage.getItem('user_id'))

const canSubmit = computed(() => {
  return formData.value.name.length >= 2 && formData.value.slug.length >= 2
})

function getInitial(name: string): string {
  return name.charAt(0).toUpperCase()
}

function generateSlug() {
  formData.value.slug = formData.value.name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

async function selectTenant(tenant: Tenant) {
  await tenantStore.selectTenant(tenant.id)
}

function editTenant(tenant: Tenant) {
  editingTenant.value = tenant
  formData.value = {
    name: tenant.name,
    slug: tenant.slug,
    description: tenant.description || '',
  }
}

function manageTenant(tenant: Tenant) {
  router.push(`/tenants/${tenant.id}`)
}

function confirmDelete(tenant: Tenant) {
  deletingTenant.value = tenant
}

function closeModal() {
  showCreateModal.value = false
  editingTenant.value = null
  formData.value = { name: '', slug: '', description: '' }
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingTenant.value) {
      await tenantStore.updateTenant(editingTenant.value.id, {
        name: formData.value.name,
        description: formData.value.description || undefined,
      })
    } else {
      await tenantStore.createTenant({
        name: formData.value.name,
        slug: formData.value.slug,
        description: formData.value.description || undefined,
      })
    }
    closeModal()
  } catch (error) {
    console.error('Failed to save tenant:', error)
  } finally {
    submitting.value = false
  }
}

async function handleDelete() {
  if (!deletingTenant.value) return

  deleting.value = true
  try {
    await tenantStore.deleteTenant(deletingTenant.value.id)
    deletingTenant.value = null
  } catch (error) {
    console.error('Failed to delete tenant:', error)
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  tenantStore.loadTenants()
})
</script>

<style scoped>
.tenants-view {
  padding: 1.5rem;
  max-width: 1200px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.header h1 {
  margin: 0;
  font-size: 1.75rem;
}

.loading {
  text-align: center;
  padding: 3rem;
  color: var(--color-text-muted);
}

.empty-state {
  text-align: center;
  padding: 3rem;
  color: var(--color-text-muted);
}

.tenants-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 1rem;
}

.tenant-card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.25rem;
  cursor: pointer;
  transition: all 0.2s;
  border: 2px solid transparent;
}

.tenant-card:hover {
  background: var(--color-background-mute);
}

.tenant-card.active {
  border-color: var(--color-primary);
}

.tenant-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.tenant-icon {
  width: 2.5rem;
  height: 2.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-primary);
  color: white;
  border-radius: 0.5rem;
  font-size: 1.125rem;
  font-weight: 600;
}

.tenant-info {
  flex: 1;
}

.tenant-name {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
}

.tenant-slug {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.tenant-status {
  font-size: 0.75rem;
  font-weight: 500;
}

.tenant-description {
  margin: 0 0 0.75rem 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  line-height: 1.5;
}

.tenant-meta {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  margin-bottom: 0.75rem;
}

.tenant-actions {
  display: flex;
  gap: 0.5rem;
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
  max-width: 500px;
  overflow: hidden;
}

.modal-sm {
  max-width: 400px;
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
  font-size: 1.25rem;
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
.form-group textarea {
  width: 100%;
  padding: 0.625rem 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  font-size: 0.875rem;
  background: var(--color-background);
  color: var(--color-text);
}

.form-group input:disabled {
  background: var(--color-background-soft);
  cursor: not-allowed;
}

.form-group textarea {
  min-height: 80px;
  resize: vertical;
}

.hint {
  margin: 0.25rem 0 0 0;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.warning {
  color: #ef4444;
  font-size: 0.875rem;
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

@media (max-width: 768px) {
  .tenants-grid {
    grid-template-columns: 1fr;
  }
}
</style>
