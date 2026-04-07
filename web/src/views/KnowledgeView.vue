<script setup lang="ts">
import AutomationTabs from '@/components/automation/AutomationTabs.vue'
import EvolutionKnowledgePane from '@/components/automation/EvolutionKnowledgePane.vue'
</script>

<template>
  <div class="knowledge-page dashboard-page-frame">
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-4">
      <AutomationTabs class="automation-tab-strip" />
      <EvolutionKnowledgePane />
    </div>
  </div>
</template>

<style scoped>
.knowledge-page {
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}
</style>
