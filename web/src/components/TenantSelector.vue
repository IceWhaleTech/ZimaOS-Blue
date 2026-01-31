<template>
  <div ref="selectorRef" class="tenant-selector">
    <button class="selector-trigger" @click="toggleDropdown">
      <div class="current-tenant">
        <span class="tenant-icon">{{ currentTenantInitial }}</span>
        <span class="tenant-name">{{ currentTenantName }}</span>
      </div>
      <span class="dropdown-arrow">{{ isOpen ? '▲' : '▼' }}</span>
    </button>

    <div v-if="isOpen" class="dropdown-menu">
      <div class="dropdown-header">Switch Workspace</div>

      <div class="tenant-list">
        <button
          v-for="tenant in tenants"
          :key="tenant.id"
          class="tenant-item"
          :class="{ active: tenant.id === currentTenantId }"
          @click="selectTenant(tenant.id)"
        >
          <span class="tenant-icon">{{ getInitial(tenant.name) }}</span>
          <div class="tenant-info">
            <span class="tenant-name">{{ tenant.name }}</span>
            <span class="tenant-slug">{{ tenant.slug }}</span>
          </div>
          <span v-if="tenant.id === currentTenantId" class="check-icon">✓</span>
        </button>
      </div>

      <div class="dropdown-footer">
        <button class="create-btn" @click="showCreateModal = true">
          + Create Workspace
        </button>
        <button class="manage-btn" @click="goToManage">
          Manage Workspaces
        </button>
      </div>
    </div>

    <!-- Create Tenant Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @click="showCreateModal = false">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h2>Create Workspace</h2>
          <button class="close-btn" @click="showCreateModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label for="tenant-name">Name</label>
            <input
              id="tenant-name"
              v-model="newTenant.name"
              type="text"
              placeholder="My Workspace"
              @input="generateSlug"
            />
          </div>
          <div class="form-group">
            <label for="tenant-slug">Slug</label>
            <input
              id="tenant-slug"
              v-model="newTenant.slug"
              type="text"
              placeholder="my-workspace"
            />
            <p class="hint">URL-friendly identifier (lowercase, no spaces)</p>
          </div>
          <div class="form-group">
            <label for="tenant-description">Description (optional)</label>
            <textarea
              id="tenant-description"
              v-model="newTenant.description"
              placeholder="What is this workspace for?"
            ></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="!canCreate || creating"
            @click="handleCreate"
          >
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTenantStore } from '@/stores/tenant'

const router = useRouter()
const tenantStore = useTenantStore()

const selectorRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)
const showCreateModal = ref(false)
const creating = ref(false)

const newTenant = ref({
  name: '',
  slug: '',
  description: '',
})

const tenants = computed(() => tenantStore.tenants)
const currentTenantId = computed(() => tenantStore.currentTenantId)
const currentTenantName = computed(() => tenantStore.currentTenant?.name || 'Select Workspace')
const currentTenantInitial = computed(() => getInitial(currentTenantName.value))

const canCreate = computed(() => {
  return newTenant.value.name.length >= 2 && newTenant.value.slug.length >= 2
})

function getInitial(name: string): string {
  return name.charAt(0).toUpperCase()
}

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

function handleClickOutside(event: MouseEvent) {
  if (selectorRef.value && !selectorRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

async function selectTenant(tenantId: string) {
  await tenantStore.selectTenant(tenantId)
  isOpen.value = false
}

function generateSlug() {
  newTenant.value.slug = newTenant.value.name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

async function handleCreate() {
  creating.value = true
  try {
    await tenantStore.createTenant({
      name: newTenant.value.name,
      slug: newTenant.value.slug,
      description: newTenant.value.description || undefined,
    })
    showCreateModal.value = false
    newTenant.value = { name: '', slug: '', description: '' }
  } catch (error) {
    console.error('Failed to create tenant:', error)
  } finally {
    creating.value = false
  }
}

function goToManage() {
  isOpen.value = false
  router.push('/tenants')
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  tenantStore.loadTenants()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.tenant-selector {
  position: relative;
}

.selector-trigger {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
  background: var(--color-background, white);
  cursor: pointer;
  transition: all 0.2s;
}

.selector-trigger:hover {
  background: var(--color-background-soft, #f9fafb);
}

.current-tenant {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.tenant-icon {
  width: 1.75rem;
  height: 1.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-primary, #3b82f6);
  color: white;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 600;
}

.tenant-name {
  font-weight: 500;
  color: var(--color-text, #1f2937);
}

.dropdown-arrow {
  font-size: 0.625rem;
  color: var(--color-text-muted, #6b7280);
}

.dropdown-menu {
  position: absolute;
  top: calc(100% + 0.5rem);
  left: 0;
  min-width: 280px;
  background: var(--color-background, white);
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.75rem;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
  z-index: 100;
  overflow: hidden;
}

.dropdown-header {
  padding: 0.75rem 1rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--color-text-muted, #6b7280);
  text-transform: uppercase;
  border-bottom: 1px solid var(--color-border, #e5e7eb);
}

.tenant-list {
  max-height: 240px;
  overflow-y: auto;
}

.tenant-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  padding: 0.75rem 1rem;
  border: none;
  background: none;
  cursor: pointer;
  transition: background 0.2s;
  text-align: left;
}

.tenant-item:hover {
  background: var(--color-background-soft, #f9fafb);
}

.tenant-item.active {
  background: var(--color-background-soft, #f3f4f6);
}

.tenant-info {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.tenant-info .tenant-name {
  font-size: 0.875rem;
}

.tenant-slug {
  font-size: 0.75rem;
  color: var(--color-text-muted, #6b7280);
}

.check-icon {
  color: var(--color-primary, #3b82f6);
  font-weight: 600;
}

.dropdown-footer {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.5rem;
  border-top: 1px solid var(--color-border, #e5e7eb);
}

.create-btn,
.manage-btn {
  width: 100%;
  padding: 0.625rem 1rem;
  border: none;
  border-radius: 0.375rem;
  background: none;
  font-size: 0.875rem;
  cursor: pointer;
  text-align: left;
  transition: background 0.2s;
}

.create-btn {
  color: var(--color-primary, #3b82f6);
  font-weight: 500;
}

.create-btn:hover {
  background: var(--color-background-soft, #f9fafb);
}

.manage-btn {
  color: var(--color-text-muted, #6b7280);
}

.manage-btn:hover {
  background: var(--color-background-soft, #f9fafb);
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
  background: var(--color-background, white);
  border-radius: 0.75rem;
  width: 90%;
  max-width: 450px;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--color-border, #e5e7eb);
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
  color: var(--color-text-muted, #6b7280);
}

.modal-body {
  padding: 1.5rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  border-top: 1px solid var(--color-border, #e5e7eb);
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
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
  font-size: 0.875rem;
}

.form-group textarea {
  min-height: 80px;
  resize: vertical;
}

.hint {
  margin: 0.25rem 0 0 0;
  font-size: 0.75rem;
  color: var(--color-text-muted, #6b7280);
}

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
  background: var(--color-primary, #3b82f6);
  color: white;
}

.btn-secondary {
  background: var(--color-background-soft, #f3f4f6);
  color: var(--color-text, #1f2937);
  border: 1px solid var(--color-border, #e5e7eb);
}
</style>
