<script setup lang="ts">
import type { Component } from "vue";

withDefaults(defineProps<{
  icon?: Component;
  variant?: "default" | "quiet" | "danger";
  disabled?: boolean;
}>(), { variant: "default", disabled: false });
</script>

<template>
  <button
    type="button"
    :disabled="disabled"
    class="inline-flex h-9 items-center justify-center gap-2 rounded-lg border px-3 text-[11px] font-medium transition disabled:cursor-not-allowed disabled:opacity-45"
    :class="{
      'border-shelf-line bg-white/[.025] text-shelf-text-soft hover:border-shelf-line-strong hover:bg-white/[.05] hover:text-shelf-text': variant === 'default',
      'border-transparent bg-transparent text-shelf-muted hover:bg-white/[.04] hover:text-shelf-text': variant === 'quiet',
      'border-red-400/20 bg-red-400/[.04] text-shelf-danger hover:border-red-400/35 hover:bg-red-400/[.08]': variant === 'danger',
    }"
  >
    <component :is="icon" v-if="icon" :size="15" :stroke-width="1.7" aria-hidden="true" />
    <slot />
  </button>
</template>
