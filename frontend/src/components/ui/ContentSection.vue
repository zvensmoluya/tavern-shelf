<script setup lang="ts">
withDefaults(defineProps<{ title: string; meta?: string }>(), { meta: "" });
</script>

<template>
  <section class="border-t border-shelf-line py-7">
    <header class="mb-4 flex items-baseline justify-between gap-4">
      <h3 class="text-[15px] font-semibold tracking-[-.01em] text-shelf-text">{{ title }}</h3>
      <span v-if="meta" class="text-[11px] text-shelf-muted">{{ meta }}</span>
    </header>
    <slot />
  </section>
</template>
