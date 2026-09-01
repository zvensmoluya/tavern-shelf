<script setup lang="ts">
import { ChevronDown, ChevronUp } from "@lucide/vue";
import { computed, ref } from "vue";

const props = withDefaults(defineProps<{
  text: string;
  limit?: number;
  label?: string;
  variant?: "lead" | "opening" | "prose";
}>(), { limit: 420, label: "展开全文", variant: "prose" });

const expanded = ref(false);
const truncated = computed(() => props.text.length > props.limit);
const preview = computed(() => {
  if (!truncated.value || expanded.value) return props.text;
  const slice = props.text.slice(0, props.limit);
  return `${slice.replace(/\s+\S*$/, "").trimEnd()}…`;
});
</script>

<template>
  <div>
    <p
      class="whitespace-pre-wrap"
      :class="{
        'text-[14px] leading-[1.85] text-shelf-text-soft': variant === 'lead',
        'rounded-r-lg border-l-2 border-shelf-line-strong bg-white/[.03] px-4 py-4 text-[13px] leading-[1.8] text-shelf-text-soft': variant === 'opening',
        'text-[13px] leading-[1.85] text-shelf-text-soft/90': variant === 'prose',
      }"
    >{{ preview }}</p>
    <button
      v-if="truncated"
      type="button"
      class="mt-3 inline-flex h-9 items-center gap-1.5 rounded-md border border-shelf-line px-3 text-[11px] text-shelf-muted transition hover:border-shelf-line-strong hover:text-shelf-text"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      {{ expanded ? "收起" : label }}
      <ChevronUp v-if="expanded" :size="13" aria-hidden="true" />
      <ChevronDown v-else :size="13" aria-hidden="true" />
    </button>
  </div>
</template>
