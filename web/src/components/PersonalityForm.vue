<template>
  <div class="modal-overlay" @click="$emit('close')">
    <div class="modal" @click.stop>
      <h3>{{ personality ? 'Edit Personality' : 'New Personality' }}</h3>

      <form @submit.prevent="handleSubmit">
        <div class="form-group">
          <label>Name</label>
          <input v-model="form.name" type="text" required />
        </div>

        <div class="form-group">
          <label>Description</label>
          <textarea v-model="form.description" rows="2"></textarea>
        </div>

        <div class="form-group">
          <label>System Prompt</label>
          <textarea v-model="form.systemPrompt" rows="4" required></textarea>
        </div>

        <div class="actions">
          <button type="submit" class="btn-save">Save</button>
          <button type="button" @click="$emit('close')" class="btn-cancel">Cancel</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { usePersonalityStore } from '@/stores/personality'
import type { Personality } from '@/api/personality'

const props = defineProps<{
  personality?: Personality | null
}>()

const emit = defineEmits<{
  save: []
  close: []
}>()

const store = usePersonalityStore()
const form = ref({
  name: '',
  description: '',
  systemPrompt: '',
})

watch(
  () => props.personality,
  (p) => {
    if (p) {
      form.value = {
        name: p.name,
        description: p.description,
        systemPrompt: p.system_prompt,
      }
    } else {
      form.value = { name: '', description: '', systemPrompt: '' }
    }
  },
  { immediate: true }
)

const handleSubmit = async () => {
  try {
    if (props.personality) {
      await store.updatePersonality(
        props.personality.id,
        form.value.name,
        form.value.description,
        form.value.systemPrompt
      )
    } else {
      await store.createPersonality(
        form.value.name,
        form.value.description,
        form.value.systemPrompt
      )
    }
    emit('save')
  } catch (e) {
    alert('Error: ' + (e instanceof Error ? e.message : 'Unknown error'))
  }
}
</script>

<style scoped>
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
  background: white;
  border-radius: 8px;
  padding: 24px;
  max-width: 500px;
  width: 90%;
}

.form-group {
  margin-bottom: 16px;
}

label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
}

input,
textarea {
  width: 100%;
  padding: 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-family: inherit;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 24px;
}

button {
  flex: 1;
  padding: 10px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-save {
  background: #2196f3;
  color: white;
}

.btn-cancel {
  background: #f0f0f0;
  color: #333;
}
</style>
