<template>
  <div class="a2ui-table">
    <table>
      <thead>
        <tr>
          <th v-for="column in columns" :key="column">{{ column }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, rowIndex) in rows" :key="rowIndex">
          <td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cell }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const columns = computed(() => {
  const cols = props.component.props?.columns
  if (Array.isArray(cols)) {
    return cols as string[]
  }
  return []
})

const rows = computed(() => {
  const rowsData = props.component.props?.rows
  if (Array.isArray(rowsData)) {
    return rowsData as unknown[][]
  }
  return []
})
</script>

<style scoped>
.a2ui-table {
  overflow-x: auto;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

th,
td {
  padding: 0.75rem 1rem;
  text-align: left;
  border-bottom: 1px solid var(--color-border, #e5e7eb);
}

th {
  font-weight: 600;
  background: var(--color-background-soft, #f9fafb);
  color: var(--color-text, #1f2937);
}

td {
  color: var(--color-text, #374151);
}

tbody tr:hover {
  background: var(--color-background-soft, #f9fafb);
}
</style>
