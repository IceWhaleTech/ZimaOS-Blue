<template>
  <div class="personality-list">
    <div v-if="loading" class="loading">Loading...</div>

    <div v-else-if="personalities.length === 0" class="empty">
      No personalities yet. Create one to get started.
    </div>

    <div v-else class="list">
      <div v-for="p in personalities" :key="p.id" class="personality-card">
        <div class="card-header">
          <h3>{{ p.name }}</h3>
          <span v-if="activeId === p.id" class="badge-active">Active</span>
        </div>
        <p class="description">{{ p.description }}</p>
        <p class="prompt">{{ p.system_prompt }}</p>
        <div class="actions">
          <button @click="$emit('edit', p)" class="btn-edit">Edit</button>
          <button
            v-if="activeId !== p.id"
            @click="$emit('activate', p.id)"
            class="btn-activate"
          >
            Activate
          </button>
          <button @click="$emit('delete', p.id)" class="btn-delete">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Personality } from '@/api/personality'

defineProps<{
  personalities: Personality[]
  activeId?: string
  loading?: boolean
}>()

defineEmits<{
  edit: [personality: Personality]
  activate: [id: string]
  delete: [id: string]
}>()
</script>

<style scoped>
.personality-list {
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.list {
  display: grid;
  gap: 16px;
}

.personality-card {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 16px;
  background: white;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.badge-active {
  background: #4caf50;
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
}

.description {
  color: #666;
  margin: 8px 0;
}

.prompt {
  background: #f5f5f5;
  padding: 8px;
  border-radius: 4px;
  font-size: 12px;
  margin: 8px 0;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

button {
  padding: 8px 12px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}

.btn-primary {
  background: #2196f3;
  color: white;
}

.btn-edit {
  background: #ff9800;
  color: white;
}

.btn-activate {
  background: #4caf50;
  color: white;
}

.btn-delete {
  background: #f44336;
  color: white;
}

.empty {
  text-align: center;
  padding: 40px;
  color: #999;
}

.loading {
  text-align: center;
  padding: 40px;
}
</style>
