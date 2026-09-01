<script setup lang="ts">
import type { Component } from "vue";

withDefaults(defineProps<{
  icon: Component;
  label: string;
  size?: number;
  active?: boolean;
}>(), { size: 18, active: false });
</script>

<template>
  <button
    type="button"
    :aria-label="label"
    :title="label"
    class="inline-flex size-9 shrink-0 items-center justify-center rounded-lg border border-shelf-line bg-white/[.025] text-shelf-muted transition hover:border-shelf-line-strong hover:bg-white/[.045] hover:text-shelf-text focus-visible:outline-offset-2"
    :class="active ? 'border-shelf-line-strong bg-shelf-soft text-shelf-text' : ''"
  >
    <component :is="icon" :size="size" :stroke-width="1.7" aria-hidden="true" />
  </button>
</template>
