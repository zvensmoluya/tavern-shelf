<script setup lang="ts">
withDefaults(defineProps<{ title: string; meta?: string }>(), { meta: "" });
</script>

<template>
  <section class="border-t border-shelf-line py-6">
    <header class="mb-3.5 flex items-baseline justify-between gap-4">
      <h3 class="text-[13px] font-semibold text-shelf-text">{{ title }}</h3>
      <span v-if="meta" class="text-[9px] text-shelf-quiet">{{ meta }}</span>
    </header>
    <slot />
  </section>
</template>
